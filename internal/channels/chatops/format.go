package chatops

import "strings"

// ChunkText splits text into chunks at newline boundaries, respecting maxLen.
func ChunkText(text string, maxLen int) []string {
	if len([]rune(text)) <= maxLen {
		return []string{text}
	}

	var chunks []string
	remaining := text
	for len(remaining) > 0 {
		runes := []rune(remaining)
		if len(runes) <= maxLen {
			chunks = append(chunks, remaining)
			break
		}
		candidate := string(runes[:maxLen])
		cutAt := maxLen
		if idx := strings.LastIndex(candidate, "\n"); idx > len(candidate)/2 {
			chunks = append(chunks, remaining[:idx+1])
			remaining = remaining[idx+1:]
			continue
		}
		chunks = append(chunks, string(runes[:cutAt]))
		remaining = string(runes[cutAt:])
	}
	return chunks
}

// TrimMention removes @botUsername from message text for cleaner agent input.
func TrimMention(text, botUsername string) string {
	// Handle both "@username" and "@username " patterns
	text = strings.ReplaceAll(text, "@"+botUsername+" ", "")
	text = strings.ReplaceAll(text, "@"+botUsername, "")
	return strings.TrimSpace(text)
}
