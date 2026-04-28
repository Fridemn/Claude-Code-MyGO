package tests

import (
	"testing"

	"claude-go/internal/ui"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		ms      int
		want    string
		comment string
	}{
		// Zero case
		{0, "0s", "zero ms"},

		// Sub-second cases (show as "0s" since we show integer seconds)
		{100, "0s", "100ms"},
		{500, "0s", "500ms"},
		{999, "0s", "999ms"},

		// Integer seconds cases (1-59 seconds)
		{1000, "1s", "1 second"},
		{5000, "5s", "5 seconds"},
		{10000, "10s", "10 seconds"},
		{30000, "30s", "30 seconds"},
		{59000, "59s", "59 seconds"},
		{59999, "59s", "just under 60 seconds"},

		// Minutes cases
		{60000, "1m0s", "60 seconds = 1 minute"},
		{90000, "1m30s", "90 seconds"},
		{120000, "2m0s", "2 minutes"},
		{3661000, "61m1s", "over an hour"},
	}

	for _, tt := range tests {
		got := ui.FormatDuration(tt.ms)
		if got != tt.want {
			t.Errorf("FormatDuration(%d) = %q, want %q (%s)", tt.ms, got, tt.want, tt.comment)
		}
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{5, "5"},
		{99, "99"},
		{100, "100"},
		{999, "999"},
		{1000, "1,000"},
		{1234, "1,234"},
		{10000, "10,000"},
		{12345, "12,345"},
		{100000, "100,000"},
		{1234567, "1,234,567"},
	}

	for _, tt := range tests {
		got := ui.FormatNumber(tt.n)
		if got != tt.want {
			t.Errorf("FormatNumber(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestGetRandomVerb(t *testing.T) {
	verb := ui.GetRandomVerb()
	if verb == "" {
		t.Error("GetRandomVerb returned empty string")
	}
	// Verb should be at least 3 characters
	if len(verb) < 3 {
		t.Errorf("GetRandomVerb returned suspiciously short verb: %q", verb)
	}
}

func TestGetSpinnerFrame(t *testing.T) {
	tests := []struct {
		timeMs int
	}{
		{0},
		{120},
		{240},
		{360},
		{1000},
		{5000},
	}

	for _, tt := range tests {
		frame := ui.GetSpinnerFrame(tt.timeMs)
		if frame == "" {
			t.Errorf("GetSpinnerFrame(%d) returned empty string", tt.timeMs)
		}
		// Frame should be one of the known spinner characters
		if len(frame) < 1 {
			t.Errorf("GetSpinnerFrame(%d) returned invalid frame: %q", tt.timeMs, frame)
		}
	}
}