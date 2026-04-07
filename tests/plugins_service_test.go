package tests

import (
	"testing"

	"claude-code-go/internal/services"
)

func TestPluginsService_PersistAndReload(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/plugins.json"
	svc := services.CreatePluginsService(path)

	svc.Add(services.Plugin{
		Name:        "demo-plugin",
		Source:      "local",
		Status:      "configured",
		Version:     "1.2.3",
		Description: "test plugin",
		Enabled:     true,
		Category:    "utility",
		Path:        "./plugins/demo",
	})

	if ok := svc.SetEnabled("demo-plugin", false); !ok {
		t.Fatalf("expected plugin enable toggle to succeed")
	}
	if ok := svc.SetStatus("demo-plugin", "loaded"); !ok {
		t.Fatalf("expected plugin status update to succeed")
	}

	reloaded := services.CreatePluginsService(path)
	var found services.Plugin
	var ok bool
	for _, plugin := range reloaded.List() {
		if plugin.Name == "demo-plugin" {
			found = plugin
			ok = true
			break
		}
	}
	if !ok {
		t.Fatalf("expected plugin to persist across reload")
	}
	if found.Enabled {
		t.Fatalf("expected persisted plugin to remain disabled")
	}
	if found.Status != "loaded" || found.Version != "1.2.3" {
		t.Fatalf("unexpected reloaded plugin: %#v", found)
	}
}

