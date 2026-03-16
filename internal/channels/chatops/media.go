package chatops

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/channels/media"
)

const defaultMediaMaxBytes int64 = 20 * 1024 * 1024 // 20MB

// fileInfo represents file metadata from a Mattermost WebSocket event.
type fileInfo struct {
	ID       string
	Name     string
	Size     int64
	MimeType string
}

// mediaItem represents a downloaded file from a Mattermost message.
type mediaItem struct {
	Type        string // "image", "audio", "document"
	FilePath    string // local temp file path
	FileName    string // original filename
	ContentType string // MIME type
}

// resolveMedia downloads and classifies files attached to a Mattermost message.
// Returns media items (with local file paths) and any extracted document text content.
func (c *Channel) resolveMedia(files []fileInfo) (items []mediaItem, extraContent string) {
	for _, f := range files {
		// Skip files exceeding size limit before download
		if f.Size > defaultMediaMaxBytes {
			slog.Warn("chatops: file too large, skipping",
				"file", f.Name, "size", f.Size, "max", defaultMediaMaxBytes)
			continue
		}

		mimeType := f.MimeType
		if mimeType == "" {
			mimeType = media.DetectMIMEType(f.Name)
		}
		mtype := media.MediaKindFromMime(mimeType)

		filePath, err := c.downloadFile(f.ID, f.Name)
		if err != nil {
			slog.Warn("chatops: file download failed",
				"file", f.Name, "file_id", f.ID, "error", err)
			continue
		}

		items = append(items, mediaItem{
			Type:        mtype,
			FilePath:    filePath,
			FileName:    f.Name,
			ContentType: mimeType,
		})

		// Extract text from document files
		if mtype == "document" {
			docContent, err := media.ExtractDocumentContent(filePath, f.Name)
			if err != nil {
				slog.Warn("chatops: document extraction failed",
					"file", f.Name, "error", err)
				continue
			}
			if extraContent != "" {
				extraContent += "\n"
			}
			extraContent += docContent
		}
	}

	return items, extraContent
}

// downloadFile downloads a file from Mattermost by its ID and saves to a temp file.
// Uses GET /api/v4/files/{file_id} with Bearer token authentication.
func (c *Channel) downloadFile(fileID, fileName string) (string, error) {
	ext := filepath.Ext(fileName)
	if ext == "" {
		ext = ".dat"
	}
	tmpFile, err := os.CreateTemp("", "chatops-file-*"+ext)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer tmpFile.Close()

	apiURL := fmt.Sprintf("%s/api/v4/files/%s", c.serverURL, url.PathEscape(fileID))
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("GET /api/v4/files/%s returned %d", fileID, resp.StatusCode)
	}

	if _, err := io.Copy(tmpFile, io.LimitReader(resp.Body, defaultMediaMaxBytes)); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}

// buildMediaTags generates content tags for chatops media items.
func buildMediaTags(items []mediaItem) string {
	var tags []string
	for _, m := range items {
		switch m.Type {
		case "image":
			tags = append(tags, "<media:image>")
		case "audio":
			tags = append(tags, "<media:audio>")
		case "document":
			if m.FileName != "" {
				tags = append(tags, fmt.Sprintf("<media:document file=%q>", m.FileName))
			} else {
				tags = append(tags, "<media:document>")
			}
		}
	}
	return strings.Join(tags, "\n")
}

// extractFileInfos extracts file metadata from a Mattermost post's metadata.files array.
func extractFileInfos(post map[string]any) []fileInfo {
	metadata, _ := post["metadata"].(map[string]any)
	if metadata == nil {
		return nil
	}

	filesRaw, _ := metadata["files"].([]any)
	if len(filesRaw) == 0 {
		return nil
	}

	var files []fileInfo
	for _, raw := range filesRaw {
		fm, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id, _ := fm["id"].(string)
		name, _ := fm["name"].(string)
		mimeType, _ := fm["mime_type"].(string)
		size, _ := fm["size"].(float64) // JSON numbers decode as float64

		if id == "" {
			continue
		}
		files = append(files, fileInfo{
			ID:       id,
			Name:     name,
			Size:     int64(size),
			MimeType: mimeType,
		})
	}

	return files
}
