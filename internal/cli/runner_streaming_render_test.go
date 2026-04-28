package cli

import (
	"testing"

	"claude-go/internal/components"
)

func TestShouldRenderStandaloneStreaming(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		entries       []components.TranscriptEntry
		busy          bool
		streamingText string
		want          bool
	}{
		{
			name:          "not busy",
			busy:          false,
			streamingText: "hello",
			want:          false,
		},
		{
			name:          "empty streaming text",
			busy:          true,
			streamingText: "",
			want:          false,
		},
		{
			name: "streaming already in entries",
			entries: []components.TranscriptEntry{
				{Kind: "assistant_streaming", Content: "hello"},
			},
			busy:          true,
			streamingText: "hello",
			want:          false,
		},
		{
			name: "no streaming entry yet",
			entries: []components.TranscriptEntry{
				{Kind: "assistant", Content: "done"},
			},
			busy:          true,
			streamingText: "hello",
			want:          true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := shouldRenderStandaloneStreaming(tt.entries, tt.busy, tt.streamingText)
			if got != tt.want {
				t.Fatalf("shouldRenderStandaloneStreaming() = %v, want %v", got, tt.want)
			}
		})
	}
}
