package services

import "testing"

func TestIsPromptTooLongMessageRecognizesCharacterLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  *CompactMessage
		want bool
	}{
		{
			name: "input characters limit",
			msg:  &CompactMessage{Content: "API error 400: input characters limit is 819200"},
			want: true,
		},
		{
			name: "input character limit",
			msg:  &CompactMessage{Content: "API error 400: input character limit is 819200"},
			want: true,
		},
		{
			name: "unrelated",
			msg:  &CompactMessage{Content: "API error 429: too many requests"},
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsPromptTooLongMessage(tt.msg); got != tt.want {
				t.Fatalf("IsPromptTooLongMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}
