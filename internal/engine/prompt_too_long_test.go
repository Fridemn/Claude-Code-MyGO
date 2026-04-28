package engine

import (
	"errors"
	"testing"
)

func TestIsPromptTooLongError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "prompt too long", err: errors.New("API error 400: prompt is too long"), want: true},
		{name: "prompt exceeds max length", err: errors.New("prompt exceeds max length"), want: true},
		{name: "input characters limit", err: errors.New("API error 400: input characters limit is 819200"), want: true},
		{name: "input character limit", err: errors.New("API error 400: input character limit is 819200"), want: true},
		{name: "other error", err: errors.New("API error 429: too many requests"), want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isPromptTooLongError(tt.err); got != tt.want {
				t.Fatalf("isPromptTooLongError() = %v, want %v", got, tt.want)
			}
		})
	}
}
