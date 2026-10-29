package media

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestScanSkipsAppleDoubleMetadata(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"film.mp4", "._film.mp4", ".hidden-but-real.MOV", "nested/clip.mkv", "nested/._clip.mkv", "__MACOSX/ghost.mp4", "notes.txt"} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := ScanWorker(dir, nil, &out); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&out)
	var paths []string
	for decoder.More() {
		var r ScanRecord
		if err := decoder.Decode(&r); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, filepath.ToSlash(r.Path))
	}
	if len(paths) != 3 || paths[0] != ".hidden-but-real.MOV" || paths[1] != "film.mp4" || paths[2] != "nested/clip.mkv" {
		t.Fatal(paths)
	}
}
