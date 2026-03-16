package chatops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
)

// Send delivers an outbound message to the Mattermost channel.
func (c *Channel) Send(_ context.Context, msg bus.OutboundMessage) error {
	if !c.IsRunning() {
		return fmt.Errorf("chatops channel not running")
	}

	channelID := msg.ChatID
	if channelID == "" {
		return fmt.Errorf("empty chat ID for chatops send")
	}

	content := msg.Content
	rootID := msg.Metadata["message_thread_id"]

	// NO_REPLY: skip empty messages
	if content == "" && len(msg.Media) == 0 {
		return nil
	}

	// Prepend @mention when replying in a group thread so the user
	// receives a Mattermost notification (matching Telegram's reply_to pattern).
	if rootID != "" && msg.Metadata["is_dm"] != "true" {
		if mentionUser := msg.Metadata["mention_username"]; mentionUser != "" {
			content = "@" + mentionUser + " " + content
		}
	}

	// Handle media attachments (upload local files by path)
	var fileIDs []string
	for _, media := range msg.Media {
		if media.URL == "" {
			continue
		}
		data, err := os.ReadFile(media.URL)
		if err != nil {
			slog.Warn("chatops: failed to read media file", "path", media.URL, "error", err)
			continue
		}
		filename := filepath.Base(media.URL)
		fileID, err := c.uploadFile(channelID, data, filename)
		if err != nil {
			slog.Warn("chatops: file upload failed", "file", filename, "error", err)
			continue
		}
		fileIDs = append(fileIDs, fileID)
	}

	// Send text in chunks
	if content != "" {
		chunks := ChunkText(content, maxMessageLen)
		for i, chunk := range chunks {
			// Attach file IDs only to the first chunk
			var ids []string
			if i == 0 {
				ids = fileIDs
			}
			if err := c.sendPost(channelID, chunk, ids, rootID); err != nil {
				return fmt.Errorf("chatops send: %w", err)
			}
		}
	} else if len(fileIDs) > 0 {
		// Files only, no text
		if err := c.sendPost(channelID, "", fileIDs, rootID); err != nil {
			return fmt.Errorf("chatops send files: %w", err)
		}
	}

	return nil
}

// sendPost sends a message to a Mattermost channel via REST API.
// rootID, when non-empty, makes this a threaded reply under the given post.
func (c *Channel) sendPost(channelID, text string, fileIDs []string, rootID string) error {
	body := map[string]any{
		"channel_id": channelID,
		"message":    text,
	}
	if len(fileIDs) > 0 {
		body["file_ids"] = fileIDs
	}
	if rootID != "" {
		body["root_id"] = rootID
	}

	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, c.serverURL+"/api/v4/posts", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST /api/v4/posts returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// uploadFile uploads a file to a Mattermost channel and returns the file ID.
func (c *Channel) uploadFile(channelID string, data []byte, filename string) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if err := w.WriteField("channel_id", channelID); err != nil {
		return "", err
	}

	part, err := w.CreateFormFile("files", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	w.Close()

	req, err := http.NewRequest(http.MethodPost, c.serverURL+"/api/v4/files", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("POST /api/v4/files returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		FileInfos []struct {
			ID string `json:"id"`
		} `json:"file_infos"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.FileInfos) == 0 {
		return "", fmt.Errorf("no file_infos in upload response")
	}

	return result.FileInfos[0].ID, nil
}

// addReaction adds an emoji reaction to a post.
func (c *Channel) addReaction(postID, emojiName string) error {
	body := map[string]string{
		"user_id":    c.botUserID,
		"post_id":    postID,
		"emoji_name": emojiName,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, c.serverURL+"/api/v4/reactions", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("POST /api/v4/reactions returned %d", resp.StatusCode)
	}

	return nil
}
