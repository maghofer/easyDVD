package engine

import (
	"testing"

	"easydvd/internal/media"
	"easydvd/internal/model"
)

func TestAutomaticWorkersRespectResourcesAndPendingTitles(t *testing.T) {
	s := model.DefaultSettings(t.TempDir())
	s.EncodingProfile = model.ProfileQuality
	videos := make([]model.Video, 20)
	for i := range videos {
		videos[i] = model.Video{Duration: 3600, Size: 1 << 30}
	}
	const plenty = uint64(1 << 50)
	if got := encodingWorkers(s, videos, 64, plenty, plenty); got != 20 {
		t.Fatal("Auto did not use pending titles on a large machine", got)
	}
	if got := encodingWorkers(s, videos, 64, 5<<30, plenty); got != 3 {
		t.Fatal("Auto exceeded available memory budget", got)
	}
	if got := encodingWorkers(s, videos, 64, 1<<30, plenty); got != 1 {
		t.Fatal("low-memory fallback", got)
	}
	if got := encodingWorkers(s, videos, 64, 0, plenty); got != 2 {
		t.Fatal("unknown-memory fallback", got)
	}
	// Exactly enough room for three lossless inputs and staged sources.
	three := s
	three.Encoders = 3
	space, _ := spaceNeeds(three, videos)
	if got := encodingWorkers(s, videos, 64, plenty, uint64(space)); got != 3 {
		t.Fatal("Auto did not account for concurrent lossless scratch", got)
	}
	if got := encodingWorkers(s, videos, 64, plenty, uint64(space-1)); got != 2 {
		t.Fatal("Auto crossed scratch-space boundary", got)
	}
	for i := 1; i < len(videos); i++ {
		videos[i].Encoded = "saved.mpg"
	}
	if got := encodingWorkers(s, videos, 64, plenty, plenty); got != 1 {
		t.Fatal("saved encodes counted as pending", got)
	}
	// Scratch reservation still includes the remaining unencoded title.
	one := s
	one.Encoders = 1
	space, _ = spaceNeeds(one, videos)
	if space < media.HCencScratchBytes(videos[0], s) {
		t.Fatal("lossless scratch reservation lost")
	}
	s.Encoders = 8
	if got := encodingWorkers(s, make([]model.Video, 12), 64, 1<<30, 0); got != 8 {
		t.Fatal("fixed worker choice was silently overridden", got)
	}
}
