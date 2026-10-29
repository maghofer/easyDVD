//go:build windows

package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyJunctionPreservesRelativeMediaAndCleanup(t *testing.T) {
	root := filepath.Join(t.TempDir(), "café Ω")
	if err := os.MkdirAll(filepath.Join(root, "generation", "disc"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "assets", "video.mpg")
	if err := os.WriteFile(source, []byte("retained source"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "alias")
	if err := directoryJunction(link, root); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(link)
	data, err := os.ReadFile(filepath.Join(link, "generation", "disc", "..", "..", "assets", "video.mpg"))
	if err != nil || string(data) != "retained source" {
		t.Fatal(string(data), err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(source); err != nil || string(data) != "retained source" {
		t.Fatal("removing alias affected source", err)
	}
}
