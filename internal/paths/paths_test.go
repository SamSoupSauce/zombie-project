package paths

import (
	"os"
	"testing"
)

func TestEnsureProjectRoot(t *testing.T) {
	root := EnsureProjectRoot()
	if root == "" {
		t.Fatalf("expected non-empty root")
	}

	// Verify that maps and scripts directories can be found from the current directory
	if !hasAssets(".") {
		t.Errorf("expected current working directory to contain maps/ and scripts/, got %s", root)
	}
}

func TestResolve(t *testing.T) {
	_ = EnsureProjectRoot()

	mapPath := Resolve("maps/zombie_outpost.tmx")
	if _, err := os.Stat(mapPath); err != nil {
		t.Errorf("failed to resolve maps/zombie_outpost.tmx: %v", err)
	}

	scriptPath := Resolve("scripts/cube_behavior.lua")
	if _, err := os.Stat(scriptPath); err != nil {
		t.Errorf("failed to resolve scripts/cube_behavior.lua: %v", err)
	}
}
