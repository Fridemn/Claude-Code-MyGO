package tests

import (
	"strings"
	"testing"

	"claude-go/internal/ui/dialogs"
)

func TestGlobalSearchStateQueryAndInsertMention(t *testing.T) {
	s := dialogs.GlobalSearchStateFor(100, 30)
	s.Search = func(query string) ([]dialogs.GlobalSearchMatch, error) {
		return []dialogs.GlobalSearchMatch{
			{File: "src/main.tsx", Line: 12, Text: "hello world"},
		}, nil
	}

	_ = s.HandleKey("h")
	_ = s.HandleKey("e")
	action := s.HandleKey("tab")
	if !action.Done {
		t.Fatalf("expected tab action to finish dialog")
	}
	if !strings.Contains(action.Insert, "@src/main.tsx#L12") {
		t.Fatalf("expected mention insert, got %q", action.Insert)
	}
}

func TestGlobalSearchStateEnterOpensMatch(t *testing.T) {
	s := dialogs.GlobalSearchStateFor(100, 30)
	s.Search = func(query string) ([]dialogs.GlobalSearchMatch, error) {
		return []dialogs.GlobalSearchMatch{
			{File: "internal/ui/model.go", Line: 80, Text: "ModelFor"},
		}, nil
	}
	_ = s.HandleKey("m")
	action := s.HandleKey("enter")
	if !action.Done || action.Open == nil {
		t.Fatalf("expected enter to open selected match")
	}
	if action.Open.File != "internal/ui/model.go" {
		t.Fatalf("unexpected open target: %#v", action.Open)
	}
}

