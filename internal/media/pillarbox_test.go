package media

import (
	"easydvd/internal/model"
	"fmt"
	"strings"
	"testing"
)

func TestPillarboxEvidenceRejectsAnyWiderFrame(t *testing.T) {
	p := &pillarboxEvidence{width: 1920, height: 1080, border: 240, safe: true}
	// Writes may split a crop diagnostic at any byte.
	_, _ = p.Write([]byte("crop=1440:1080:24"))
	_, _ = p.Write([]byte("0:0\n"))
	if !p.safe || p.visible != 1 {
		t.Fatal(p)
	}
	for _, crop := range []string{"1440:1080:240:0", "0:0:0:0", "-1918:-1078:1918:1078"} {
		_, _ = fmt.Fprintf(p, "crop=%s\n", crop)
	}
	if !p.safe {
		t.Fatal("black frames should not invalidate established borders")
	}
	_, _ = p.Write([]byte("crop=1920:1080:0:0\n"))
	if p.safe {
		t.Fatal("one wider frame must veto cropping")
	}
}

func TestPillarboxKeepsContentAspectAndCacheIdentity(t *testing.T) {
	s := model.DefaultSettings(t.TempDir())
	v := model.Video{Width: 1920, Height: 1080, DAR: 16.0 / 9, CropLeft: 240, CropRight: 240, FPS: 25}
	filter, _ := VideoFilter(v, s)
	if v.Aspect() != "4:3" || v.ContentDAR() != 4.0/3 || !strings.HasPrefix(filter, "crop=1440:1080:240:0,scale=720:576:") {
		t.Fatal(filter, v.ContentDAR())
	}
	before := v.ArtifactKey(s)
	v.CropLeft, v.CropRight = 0, 0
	if before == v.ArtifactKey(s) {
		t.Fatal("changed cropping reused an encoded artifact")
	}
}
