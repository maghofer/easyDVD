package media

import (
	"context"
	"easydvd/internal/model"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckpointIntegrityAndInvalidation(t *testing.T) {
	ctx := context.Background()
	s := model.DefaultSettings(t.TempDir())
	path := filepath.Join(t.TempDir(), "video.mpg")
	if err := os.WriteFile(path, []byte("completed encoded artifact"), 0644); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(path)
	h, _ := HashFile(ctx, path)
	v := model.Video{Path: "source.mp4", Size: 100, ModTime: 1, Encoded: path, EncodedSize: st.Size(), EncodedHash: h}
	v.EncodingKey = s.EncodingKey() + v.Fingerprint()
	if !ValidArtifact(ctx, v, s) {
		t.Fatal("valid completed artifact rejected")
	}
	changed := v
	changed.ModTime = 2
	if ValidArtifact(ctx, changed, s) {
		t.Fatal("changed source accepted")
	}
	if err := os.WriteFile(path, []byte("corrupted encoded artifact"), 0644); err != nil {
		t.Fatal(err)
	}
	if ValidArtifact(ctx, v, s) {
		t.Fatal("corrupted artifact accepted")
	}
}
