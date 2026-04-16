package chatops

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/channels"
)

// handleEvent routes WebSocket events by type.
// Mattermost Chatops clusters relay cross-node events with a
// "custom_websocket-event_" prefix; strip it so standard handlers match.
func (c *Channel) handleEvent(ctx context.Context, event map[string]any) {
	eventType, _ := event["event"].(string)
	eventType = strings.TrimPrefix(eventType, "custom_websocket-event_")
	switch eventType {
	case "posted":
		c.handlePosted(ctx, event)
	case "hello":
		slog.Info("chatops: WebSocket connected (hello)")
	}
}

// handlePosted processes a new message event from Mattermost.
func (c *Channel) handlePosted(ctx context.Context, event map[string]any) {
	data, _ := event["data"].(map[string]any)
	if data == nil {
		return
	}

	// Parse the nested post JSON string
	postStr, _ := data["post"].(string)
	if postStr == "" {
		return
	}

	var post map[string]any
	if err := json.Unmarshal([]byte(postStr), &post); err != nil {
		return
	}

	userID, _ := post["user_id"].(string)
	channelID, _ := post["channel_id"].(string)
	message, _ := post["message"].(string)
	postID, _ := post["id"].(string)
	rootID, _ := post["root_id"].(string)

	// Skip self-sent messages
	if userID == "" || userID == c.botUserID {
		return
	}

	// Extract file attachments from post metadata
	files := extractFileInfos(post)

	// Skip messages with no content AND no files
	if message == "" && len(files) == 0 {
		return
	}

	// Determine channel type: D = DM, O/P/G = group
	channelType, _ := data["channel_type"].(string)
	isDM := channelType == "D"
	peerKind := "group"
	if isDM {
		peerKind = "direct"
	}

	// Resolve display name and Mattermost username (for @mention)
	displayName, mmUsername := c.resolveUser(userID)
	displayName = strings.ReplaceAll(displayName, "|", "_")
	compoundSenderID := fmt.Sprintf("%s|%s", userID, displayName)

	// Policy check
	if isDM {
		if !c.checkDMPolicy(ctx, userID, channelID) {
			return
		}
	} else {
		if !c.checkGroupPolicy(ctx,userID, channelID) {
			return
		}
	}

	content := message

	// Compute thread root for reply: if already in a thread use its root, otherwise reply under this post.
	replyRootID := rootID
	if replyRootID == "" {
		replyRootID = postID
	}

	// Session isolation: each thread gets its own session key.
	localKey := channelID
	if replyRootID != "" {
		localKey = fmt.Sprintf("%s:thread:%s", channelID, replyRootID)
	}

	// Mention gating in groups (BEFORE file download to avoid wasting bandwidth)
	if !isDM && c.requireMention {
		mentioned := strings.Contains(content, "@"+c.botUsername)
		if !mentioned {
			c.groupHistory.Record(localKey, channels.HistoryEntry{
				Sender:    displayName,
				SenderID:  userID,
				Body:      content,
				Timestamp: time.Now(),
			}, c.historyLimit)
			slog.Debug("chatops group message recorded (no mention)",
				"channel_id", channelID, "user", displayName)
			return
		}
	}

	// Strip mention from content
	content = TrimMention(content, c.botUsername)
	if content == "" && len(files) == 0 {
		return
	}

	// Process file attachments (deferred until after mention gate to avoid
	// downloading files for unmentioned messages, matching Telegram pattern)
	var mediaPaths []string
	if len(files) > 0 {
		items, docContent := c.resolveMedia(files)
		for _, item := range items {
			if item.FilePath != "" {
				mediaPaths = append(mediaPaths, item.FilePath)
			}
		}
		mediaTags := buildMediaTags(items)
		if mediaTags != "" {
			if content != "" {
				content = mediaTags + "\n\n" + content
			} else {
				content = mediaTags
			}
		}
		if docContent != "" {
			if content != "" {
				content = content + "\n\n" + docContent
			} else {
				content = docContent
			}
		}
	}

	slog.Debug("chatops message received",
		"sender_id", userID, "channel_id", channelID,
		"is_dm", isDM, "has_files", len(files) > 0,
		"preview", channels.Truncate(content, 50))

	// Build final content with group history context
	finalContent := content
	if peerKind == "group" {
		annotated := fmt.Sprintf("[From: %s]\n%s", displayName, content)
		if c.historyLimit > 0 {
			finalContent = c.groupHistory.BuildContext(localKey, annotated, c.historyLimit)
		} else {
			finalContent = annotated
		}
	}

	metadata := map[string]string{
		"user_id":    userID,
		"username":   displayName,
		"channel_id": channelID,
		"is_dm":      fmt.Sprintf("%t", isDM),
		"local_key":  localKey,
	}
	if mmUsername != "" {
		metadata["mention_username"] = mmUsername
	}
	if replyRootID != "" {
		metadata["message_thread_id"] = replyRootID
		// Track active thread per channel so Send() can thread tool-sent
		// messages that arrive without message_thread_id in metadata.
		if !isDM {
			c.activeThread.Store(channelID, replyRootID)
		}
	}

	c.HandleMessage(compoundSenderID, channelID, finalContent, mediaPaths, metadata, peerKind)

	if peerKind == "group" {
		c.groupHistory.Clear(localKey)
	}
}

// resolveUser fetches and caches the user's display name and Mattermost username.
// The username is the login handle (e.g. "vinhngh-runsystem.net") used for @mentions.
func (c *Channel) resolveUser(userID string) (displayName, username string) {
	if v, ok := c.userCache.Load(userID); ok {
		cu := v.(cachedUser)
		if time.Since(cu.fetchedAt) < userCacheTTL {
			return cu.displayName, cu.username
		}
	}

	user, err := c.apiGet("/api/v4/users/" + userID)
	if err != nil {
		slog.Debug("chatops: failed to resolve user", "user_id", userID, "error", err)
		return userID, ""
	}

	firstName, _ := user["first_name"].(string)
	lastName, _ := user["last_name"].(string)
	mmUsername, _ := user["username"].(string)

	name := strings.TrimSpace(firstName + " " + lastName)
	if name == "" {
		name = mmUsername
	}
	if name == "" {
		name = userID
	}

	c.userCache.Store(userID, cachedUser{displayName: name, username: mmUsername, fetchedAt: time.Now()})
	return name, mmUsername
}

// ListGroupMembers returns members of a Mattermost channel for on-demand @mention resolution.
// Uses /api/v4/users?in_channel={id}. Capped at 50 members; excludes the bot. Cached for 1 hour.
func (c *Channel) ListGroupMembers(_ context.Context, chatID string) ([]channels.GroupMember, error) {
	if v, ok := c.memberCache.Load(chatID); ok {
		cm := v.(cachedMemberList)
		if time.Since(cm.fetchedAt) < userCacheTTL {
			return cm.members, nil
		}
	}

	path := fmt.Sprintf("/api/v4/users?in_channel=%s&page=0&per_page=50&sort=status", chatID)
	users, err := c.apiGetArray(path)
	if err != nil {
		return nil, fmt.Errorf("fetch channel members: %w", err)
	}

	var members []channels.GroupMember
	for _, u := range users {
		uid, _ := u["id"].(string)
		if uid == c.botUserID {
			continue
		}
		username, _ := u["username"].(string)
		firstName, _ := u["first_name"].(string)
		lastName, _ := u["last_name"].(string)

		name := strings.TrimSpace(firstName + " " + lastName)
		if name == "" {
			name = username
		}
		members = append(members, channels.GroupMember{
			MemberID: username,
			Name:     name,
		})
	}

	c.memberCache.Store(chatID, cachedMemberList{members: members, fetchedAt: time.Now()})
	return members, nil
}

// --- Policy checks (same pattern as Slack) ---

func (c *Channel) checkDMPolicy(ctx context.Context, senderID, channelID string) bool {
	switch c.dmPolicy {
	case "disabled":
		return false
	case "open":
		return true
	case "allowlist":
		return c.HasAllowList() && c.IsAllowed(senderID)
	default: // "pairing"
		if c.pairingService != nil {
			paired, err := c.pairingService.IsPaired(ctx, senderID, c.Name())
			if err != nil {
				slog.Warn("security.pairing_check_failed, assuming paired (fail-open)",
					"sender_id", senderID, "channel", c.Name(), "error", err)
				return true
			}
			if paired {
				return true
			}
		}
		if c.HasAllowList() && c.IsAllowed(senderID) {
			return true
		}
		c.sendPairingReply(ctx,senderID, channelID)
		return false
	}
}

func (c *Channel) checkGroupPolicy(ctx context.Context, senderID, channelID string) bool {
	switch c.groupPolicy {
	case "disabled":
		return false
	case "open":
		return true
	case "allowlist":
		if !c.HasAllowList() {
			return false
		}
		return c.IsAllowed(senderID) || c.IsAllowed(channelID)
	default: // "pairing"
		if c.HasAllowList() && c.IsAllowed(senderID) {
			return true
		}
		if _, cached := c.approvedGroups.Load(channelID); cached {
			return true
		}
		groupSenderID := fmt.Sprintf("group:%s", channelID)
		if c.pairingService != nil {
			paired, err := c.pairingService.IsPaired(ctx, groupSenderID, c.Name())
			if err != nil {
				slog.Warn("security.pairing_check_failed, assuming paired (fail-open)",
					"group_sender", groupSenderID, "channel", c.Name(), "error", err)
				paired = true
			}
			if paired {
				c.approvedGroups.Store(channelID, true)
				return true
			}
		}
		c.sendPairingReply(ctx,groupSenderID, channelID)
		return false
	}
}

func (c *Channel) sendPairingReply(ctx context.Context, senderID, channelID string) {
	if c.pairingService == nil {
		return
	}

	if lastSent, ok := c.pairingDebounce.Load(senderID); ok {
		if time.Since(lastSent.(time.Time)) < pairingDebounceTime {
			return
		}
	}

	code, err := c.pairingService.RequestPairing(ctx, senderID, c.Name(), channelID, "default", nil)
	if err != nil {
		slog.Warn("chatops: failed to request pairing code", "error", err)
		return
	}

	var msg string
	if strings.HasPrefix(senderID, "group:") {
		msg = fmt.Sprintf("This channel is not authorized to use this bot.\n\n"+
			"An admin can approve via CLI:\n  goclaw pairing approve %s\n\n"+
			"Or approve via the GoClaw web UI (Pairing section).", code)
	} else {
		msg = fmt.Sprintf("GoClaw: access not configured.\n\nYour user ID: %s\n\nPairing code: %s\n\nAsk the bot owner to approve with:\n  goclaw pairing approve %s",
			senderID, code, code)
	}

	if err := c.sendPost(channelID, msg, nil, ""); err != nil {
		slog.Warn("chatops: failed to send pairing reply",
			"channel_id", channelID, "error", err)
	}
	c.pairingDebounce.Store(senderID, time.Now())
}

