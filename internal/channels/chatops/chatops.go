// Package chatops implements a Mattermost-compatible channel for ChatOps platforms.
// Uses raw HTTP + gorilla/websocket (no SDK) for API v4 compatibility.
// Block reply only (no streaming/edit-in-place in v1).
package chatops

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/channels"
	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

const (
	maxMessageLen       = 16383 // Mattermost default post max length
	pairingDebounceTime = 60 * time.Second
	wsReconnectMax      = 10
	wsPingInterval      = 30 * time.Second
	userCacheTTL        = 1 * time.Hour
)

// Channel connects to a Mattermost-compatible server via WebSocket + REST API.
type Channel struct {
	*channels.BaseChannel
	serverURL      string
	token          string
	botUserID      string
	botUsername     string
	httpClient     *http.Client
	requireMention bool
	blockReply     *bool

	pairingDebounce sync.Map // senderID -> time.Time
	approvedGroups  sync.Map // channelID -> true
	userCache    sync.Map // userID -> cachedUser
	memberCache  sync.Map // channelID -> cachedMembers
	activeThread sync.Map // channelID -> replyRootID (fallback thread for tool-sent messages)

	pairingService store.PairingStore
	groupHistory   *channels.PendingHistory
	historyLimit   int
	dmPolicy       string
	groupPolicy    string
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

type cachedUser struct {
	displayName string
	username    string // Mattermost login handle (e.g. "vinhngh-runsystem.net") for @mention
	fetchedAt   time.Time
}

// cachedMembers stores resolved channel member list for group @mention injection.
type cachedMembers struct {
	list      string // formatted: "Alice (@alice), Bob (@bob)"
	fetchedAt time.Time
}

// Compile-time interface assertions.
var _ channels.Channel = (*Channel)(nil)
var _ channels.BlockReplyChannel = (*Channel)(nil)
var _ channels.PendingCompactable = (*Channel)(nil)

// New creates a new ChatOps channel from config.
func New(cfg config.ChatOpsConfig, msgBus *bus.MessageBus, pairingSvc store.PairingStore, pendingStore store.PendingMessageStore) (*Channel, error) {
	if cfg.ServerURL == "" {
		return nil, fmt.Errorf("chatops server_url is required")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("chatops token is required")
	}

	base := channels.NewBaseChannel(channels.TypeChatOps, msgBus, cfg.AllowFrom)
	base.ValidatePolicy(cfg.DMPolicy, cfg.GroupPolicy)

	requireMention := true
	if cfg.RequireMention != nil {
		requireMention = *cfg.RequireMention
	}

	historyLimit := cfg.HistoryLimit
	if historyLimit == 0 {
		historyLimit = channels.DefaultGroupHistoryLimit
	}

	dmPolicy := cfg.DMPolicy
	if dmPolicy == "" {
		dmPolicy = "pairing"
	}
	groupPolicy := cfg.GroupPolicy
	if groupPolicy == "" {
		groupPolicy = "allowlist"
	}

	return &Channel{
		BaseChannel:    base,
		serverURL:      strings.TrimRight(cfg.ServerURL, "/"),
		token:          cfg.Token,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		requireMention: requireMention,
		blockReply:     cfg.BlockReply,
		pairingService: pairingSvc,
		groupHistory:   channels.MakeHistory(channels.TypeChatOps, pendingStore),
		historyLimit:   historyLimit,
		dmPolicy:       dmPolicy,
		groupPolicy:    groupPolicy,
	}, nil
}

// BlockReplyEnabled returns the per-channel block_reply override.
func (c *Channel) BlockReplyEnabled() *bool { return c.blockReply }

// SetPendingCompaction configures LLM-based auto-compaction for pending messages.
func (c *Channel) SetPendingCompaction(cfg *channels.CompactionConfig) {
	c.groupHistory.SetCompactionConfig(cfg)
}

// Start authenticates and connects the WebSocket listener.
func (c *Channel) Start(ctx context.Context) error {
	c.groupHistory.StartFlusher()

	// Validate token via /api/v4/users/me
	me, err := c.apiGet("/api/v4/users/me")
	if err != nil {
		return fmt.Errorf("chatops auth failed (GET /api/v4/users/me): %w", err)
	}

	c.botUserID, _ = me["id"].(string)
	c.botUsername, _ = me["username"].(string)
	if c.botUserID == "" {
		return fmt.Errorf("chatops: could not extract user ID from /api/v4/users/me")
	}

	wsCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.listenLoop(wsCtx)
	}()

	c.SetRunning(true)
	slog.Info("chatops channel started", "user", c.botUsername, "server", c.serverURL)
	return nil
}

// Stop gracefully shuts down the channel.
func (c *Channel) Stop(_ context.Context) error {
	c.groupHistory.StopFlusher()
	slog.Info("stopping chatops channel")
	c.SetRunning(false)

	if c.cancel != nil {
		c.cancel()
	}

	doneCh := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:
	case <-time.After(10 * time.Second):
		slog.Warn("chatops channel stop timed out after 10s")
	}

	return nil
}

// listenLoop connects to the Mattermost WebSocket and processes events with reconnect backoff.
func (c *Channel) listenLoop(ctx context.Context) {
	defer c.SetRunning(false)

	attempt := 0
	for {
		if ctx.Err() != nil {
			return
		}

		if attempt >= wsReconnectMax {
			slog.Error("chatops: max WS reconnect attempts reached, stopping")
			return
		}

		conn, err := c.connectWS(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			delay := min(time.Duration(1<<uint(attempt+1))*time.Second, 60*time.Second)
			slog.Warn("chatops: WS connect failed, retrying", "error", err, "delay", delay)
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			attempt++
			continue
		}

		// Reset attempt counter on successful connect
		attempt = 0

		if c.runWSLoop(ctx, conn) {
			// Clean disconnect requested
			return
		}

		// Connection lost, retry
		slog.Warn("chatops: WS connection lost, reconnecting")
		attempt++
	}
}

// connectWS establishes a WebSocket connection with Bearer token authentication.
// Uses Authorization header for the WS upgrade handshake (standard for bot/PAT tokens).
// Falls back to authentication_challenge for older Mattermost versions.
func (c *Channel) connectWS(ctx context.Context) (*websocket.Conn, error) {
	wsURL := c.serverURL + "/api/v4/websocket"
	// Convert http(s) to ws(s)
	if len(wsURL) > 5 && wsURL[:5] == "https" {
		wsURL = "wss" + wsURL[5:]
	} else if len(wsURL) > 4 && wsURL[:4] == "http" {
		wsURL = "ws" + wsURL[4:]
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	// Authenticate via Authorization header during WS upgrade handshake.
	// This is the recommended method for bot tokens and personal access tokens.
	headers := http.Header{
		"Authorization": []string{"Bearer " + c.token},
	}
	conn, _, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	// Also send authentication_challenge as fallback for older Mattermost versions
	// that don't support header-based WS auth.
	authMsg := map[string]any{
		"seq":    1,
		"action": "authentication_challenge",
		"data":   map[string]any{"token": c.token},
	}
	if err := conn.WriteJSON(authMsg); err != nil {
		conn.Close()
		return nil, fmt.Errorf("auth challenge: %w", err)
	}

	return conn, nil
}

// runWSLoop reads WS messages until disconnect. Returns true if context cancelled (clean shutdown).
func (c *Channel) runWSLoop(ctx context.Context, conn *websocket.Conn) bool {
	defer conn.Close()

	// Ping keepalive goroutine — wait for it on exit to prevent leak
	pingDone := make(chan struct{})
	defer func() { <-pingDone }()
	go func() {
		defer close(pingDone)
		ticker := time.NewTicker(wsPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	for {
		if ctx.Err() != nil {
			return true
		}

		_, rawMsg, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return true
			}
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return false
			}
			slog.Debug("chatops: WS read error", "error", err)
			return false
		}

		var event map[string]any
		if json.Unmarshal(rawMsg, &event) != nil {
			continue
		}

		c.handleEvent(event)
	}
}

// apiGet performs a GET request to the Mattermost API.
func (c *Channel) apiGet(path string) (map[string]any, error) {
	req, err := http.NewRequest(http.MethodGet, c.serverURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("401 unauthorized — token expired or invalid")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// apiGetArray performs a GET request that returns a JSON array.
func (c *Channel) apiGetArray(path string) ([]map[string]any, error) {
	req, err := http.NewRequest(http.MethodGet, c.serverURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}
