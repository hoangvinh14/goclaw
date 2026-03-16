package chatops

import (
	"reflect"
	"testing"
)

func TestExtractFileInfos(t *testing.T) {
	tests := []struct {
		name     string
		post     map[string]any
		expected []fileInfo
	}{
		{
			name: "post with one PDF file",
			post: map[string]any{
				"message": "hello",
				"metadata": map[string]any{
					"files": []any{
						map[string]any{
							"id":        "abc123",
							"name":      "document.pdf",
							"size":      float64(172674),
							"mime_type": "application/pdf",
						},
					},
				},
			},
			expected: []fileInfo{
				{ID: "abc123", Name: "document.pdf", Size: 172674, MimeType: "application/pdf"},
			},
		},
		{
			name: "post with multiple files",
			post: map[string]any{
				"message": "check these files",
				"metadata": map[string]any{
					"files": []any{
						map[string]any{
							"id":        "img1",
							"name":      "photo.jpg",
							"size":      float64(50000),
							"mime_type": "image/jpeg",
						},
						map[string]any{
							"id":        "doc1",
							"name":      "report.xlsx",
							"size":      float64(100000),
							"mime_type": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
						},
					},
				},
			},
			expected: []fileInfo{
				{ID: "img1", Name: "photo.jpg", Size: 50000, MimeType: "image/jpeg"},
				{ID: "doc1", Name: "report.xlsx", Size: 100000, MimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
			},
		},
		{
			name:     "post with no metadata",
			post:     map[string]any{"message": "hello"},
			expected: nil,
		},
		{
			name: "post with empty files array",
			post: map[string]any{
				"metadata": map[string]any{
					"files": []any{},
				},
			},
			expected: nil,
		},
		{
			name: "file without id is skipped",
			post: map[string]any{
				"metadata": map[string]any{
					"files": []any{
						map[string]any{
							"name":      "orphan.txt",
							"size":      float64(100),
							"mime_type": "text/plain",
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "metadata without files key",
			post: map[string]any{
				"metadata": map[string]any{
					"other": "data",
				},
			},
			expected: nil,
		},
		{
			name: "non-map entry in files array is skipped",
			post: map[string]any{
				"metadata": map[string]any{
					"files": []any{
						"not a map",
						map[string]any{
							"id":   "valid",
							"name": "good.txt",
						},
					},
				},
			},
			expected: []fileInfo{
				{ID: "valid", Name: "good.txt", Size: 0, MimeType: ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractFileInfos(tt.post)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("extractFileInfos() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestBuildMediaTags(t *testing.T) {
	tests := []struct {
		name     string
		items    []mediaItem
		expected string
	}{
		{
			name:     "single image",
			items:    []mediaItem{{Type: "image"}},
			expected: "<media:image>",
		},
		{
			name:     "single document with name",
			items:    []mediaItem{{Type: "document", FileName: "test.pdf"}},
			expected: `<media:document file="test.pdf">`,
		},
		{
			name:     "single document without name",
			items:    []mediaItem{{Type: "document"}},
			expected: "<media:document>",
		},
		{
			name:     "single audio",
			items:    []mediaItem{{Type: "audio"}},
			expected: "<media:audio>",
		},
		{
			name: "multiple mixed types",
			items: []mediaItem{
				{Type: "image"},
				{Type: "document", FileName: "report.xlsx"},
				{Type: "audio"},
			},
			expected: "<media:image>\n<media:document file=\"report.xlsx\">\n<media:audio>",
		},
		{
			name:     "empty items",
			items:    nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildMediaTags(tt.items)
			if got != tt.expected {
				t.Errorf("buildMediaTags() = %q, want %q", got, tt.expected)
			}
		})
	}
}
