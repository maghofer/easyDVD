package engine

import (
	"testing"

	"easydvd/internal/media"
	"easydvd/internal/model"
)

func TestBlurayScratchFollowsContentRatherThanBlankCapacity(t *testing.T) {
	s := model.DefaultSettings(t.TempDir())
	s.DiscFormat, s.Media, s.Encoders = "Blu-ray", "BD-25", 1
	videos := []model.Video{{ID: "a", Duration: 2, Size: 300000}, {ID: "b", Duration: 2, Size: 300000}, {ID: "c", Duration: 2, Size: 300000}}
	work25, output25 := spaceNeeds(s, videos)
	if work25 > 3<<30 || output25 <= 64<<20 {
		t.Fatal("short Blu-ray job reserves blank-disc capacity or omits structure", work25, output25)
	}
	s.Media = "BD-50"
	work50, output50 := spaceNeeds(s, videos)
	if work50 != work25 || output50 != output25 {
		t.Fatal("unchanged short content reserves more space on larger media", work50, work25, output50, output25)
	}
	// Existing prepared files must still count towards both authoring copies
	// and the final output; reducing temporary encode space cannot hide them.
	videos[0].Encoded, videos[0].EncodedSize = "prepared.m2ts", 20_000_000_000
	workLarge, outputLarge := spaceNeeds(s, videos)
	if workLarge < 40_000_000_000 || outputLarge < videos[0].EncodedSize {
		t.Fatal("prepared media lost from reservation", workLarge, outputLarge)
	}
	for i := range videos {
		videos[i].Encoded, videos[i].EncodedSize = "prepared.m2ts", 20_000_000_000
	}
	workSet, outputSet := spaceNeeds(s, videos)
	if workSet < 3*outputSet {
		t.Fatal("retained multi-disc authoring or same-volume output omitted", workSet, outputSet)
	}
}

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
