package chatops

import (
	"encoding/json"
	"fmt"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/channels"
	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// chatopsCreds maps the credentials JSON from the channel_instances table.
type chatopsCreds struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
}

// chatopsInstanceConfig maps the non-secret config JSONB from the channel_instances table.
type chatopsInstanceConfig struct {
	DMPolicy       string   `json:"dm_policy,omitempty"`
	GroupPolicy    string   `json:"group_policy,omitempty"`
	AllowFrom      []string `json:"allow_from,omitempty"`
	BlockedGroups  []string `json:"blocked_groups,omitempty"`
	RequireMention *bool    `json:"require_mention,omitempty"`
	HistoryLimit   int      `json:"history_limit,omitempty"`
	BlockReply     *bool    `json:"block_reply,omitempty"`
}

// Factory creates a ChatOps channel from DB instance data.
func Factory(name string, creds json.RawMessage, cfg json.RawMessage,
	msgBus *bus.MessageBus, pairingSvc store.PairingStore) (channels.Channel, error) {

	var c chatopsCreds
	if len(creds) > 0 {
		if err := json.Unmarshal(creds, &c); err != nil {
			return nil, fmt.Errorf("decode chatops credentials: %w", err)
		}
	}
	if c.ServerURL == "" {
		return nil, fmt.Errorf("chatops server_url is required")
	}
	if c.Token == "" {
		return nil, fmt.Errorf("chatops token is required")
	}

	var ic chatopsInstanceConfig
	if len(cfg) > 0 {
		if err := json.Unmarshal(cfg, &ic); err != nil {
			return nil, fmt.Errorf("decode chatops config: %w", err)
		}
	}

	chatopsCfg := config.ChatOpsConfig{
		Enabled:        true,
		ServerURL:      c.ServerURL,
		Token:          c.Token,
		AllowFrom:      ic.AllowFrom,
		BlockedGroups:  ic.BlockedGroups,
		DMPolicy:       ic.DMPolicy,
		GroupPolicy:    ic.GroupPolicy,
		RequireMention: ic.RequireMention,
		HistoryLimit:   ic.HistoryLimit,
		BlockReply:     ic.BlockReply,
	}

	// Secure default: DB instances default to "pairing" for groups.
	if chatopsCfg.GroupPolicy == "" {
		chatopsCfg.GroupPolicy = "pairing"
	}

	ch, err := New(chatopsCfg, msgBus, pairingSvc, nil)
	if err != nil {
		return nil, err
	}
	ch.SetName(name)
	return ch, nil
}

// FactoryWithPendingStore returns a ChannelFactory with persistent history support.
func FactoryWithPendingStore(pendingStore store.PendingMessageStore) channels.ChannelFactory {
	return func(name string, creds json.RawMessage, cfg json.RawMessage,
		msgBus *bus.MessageBus, pairingSvc store.PairingStore) (channels.Channel, error) {

		var c chatopsCreds
		if len(creds) > 0 {
			if err := json.Unmarshal(creds, &c); err != nil {
				return nil, fmt.Errorf("decode chatops credentials: %w", err)
			}
		}
		if c.ServerURL == "" {
			return nil, fmt.Errorf("chatops server_url is required")
		}
		if c.Token == "" {
			return nil, fmt.Errorf("chatops token is required")
		}

		var ic chatopsInstanceConfig
		if len(cfg) > 0 {
			if err := json.Unmarshal(cfg, &ic); err != nil {
				return nil, fmt.Errorf("decode chatops config: %w", err)
			}
		}

		chatopsCfg := config.ChatOpsConfig{
			Enabled:        true,
			ServerURL:      c.ServerURL,
			Token:          c.Token,
			AllowFrom:      ic.AllowFrom,
			BlockedGroups:  ic.BlockedGroups,
			DMPolicy:       ic.DMPolicy,
			GroupPolicy:    ic.GroupPolicy,
			RequireMention: ic.RequireMention,
			HistoryLimit:   ic.HistoryLimit,
			BlockReply:     ic.BlockReply,
		}

		if chatopsCfg.GroupPolicy == "" {
			chatopsCfg.GroupPolicy = "pairing"
		}

		ch, err := New(chatopsCfg, msgBus, pairingSvc, pendingStore)
		if err != nil {
			return nil, err
		}
		ch.SetName(name)
		return ch, nil
	}
}
