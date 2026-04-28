package api

import "testing"

func TestIsLengthErrorRecognizesCharacterLimit(t *testing.T) {
	t.Parallel()

	err := &APIError{
		StatusCode: 400,
		Message:    "input characters limit is 819200",
	}
	if !IsLengthError(err) {
		t.Fatal("expected character-limit error to be treated as length error")
	}
}

func TestClassifyAPIErrorRecognizesCharacterLimit(t *testing.T) {
	t.Parallel()

	err := &APIError{
		StatusCode: 400,
		Message:    `API error 400: input characters limit is 819200`,
	}
	if got := ClassifyAPIError(err); got != ClassificationPromptTooLong {
		t.Fatalf("ClassifyAPIError() = %q, want %q", got, ClassificationPromptTooLong)
	}
}
