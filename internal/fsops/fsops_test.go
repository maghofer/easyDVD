package fsops

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestResetOnlyOwnedPartialDirectory(t *testing.T) {
	root := t.TempDir()
	id := "0123456789abcdef01234567"
	sentinel := filepath.Join(root, "valuable.iso")
	if err := os.WriteFile(sentinel, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{root, filepath.Join(root, "unrelated"), filepath.Join(root, ".easydvd-other.partial")} {
		if err := Worker([]string{"reset-partial", path, id}, io.Discard); err == nil {
			t.Fatalf("unsafe reset accepted: %s", path)
		}
	}
	partial := filepath.Join(root, ".easydvd-"+id+".partial")
	if err := os.MkdirAll(filepath.Join(partial, "menus"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(partial, "menus", "old.png"), []byte("stale preview"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Worker([]string{"reset-partial", partial, id}, io.Discard); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(partial)
	if err != nil || len(entries) != 0 {
		t.Fatal("stale publication files survived", entries, err)
	}
	if b, err := os.ReadFile(sentinel); err != nil || string(b) != "keep" {
		t.Fatal("unrelated output changed", err)
	}
}
