package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"easydvd/internal/media"
	"easydvd/internal/model"
	"easydvd/internal/process"
)

// Reusing tiny pre-encoded titles isolates navigation scaling from encoding
// duration. The former jump-pad table overflowed at 36 titles; 99 single-title
// pages also exercise the full supported menu-PGC range.
func TestAuthorManyTitles(t *testing.T) {
	if os.Getenv("EASYDVD_INTEGRATION") != "1" {
		t.Skip("set EASYDVD_INTEGRATION=1 to run real DVD authoring")
	}
	tools := media.FindTools()
	if err := tools.Require(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	root := t.TempDir()
	seed := filepath.Join(root, "seed.mpg")
	args := []string{"-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=720x576:rate=25", "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo", "-t", "2", "-target", "pal-dvd", "-aspect", "4:3", "-c:a", "ac3", "-ac", "2", seed}
	if err := process.Run(ctx, tools.FFmpeg, args, process.Options{}); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct{ titles, perPage int }{{36, 10}, {99, 1}} {
		t.Run(fmt.Sprintf("%d-titles-%d-per-page", fixture.titles, fixture.perPage), func(t *testing.T) {
			dir := filepath.Join(root, fmt.Sprintf("disc-%d", fixture.titles))
			s := model.DefaultSettings(root)
			s.TitlesPerPage = fixture.perPage
			job := model.Job{Title: "Many titles", Settings: s}
			disc := model.Disc{Number: 1}
			for i := 1; i <= fixture.titles; i++ {
				id := fmt.Sprintf("title-%03d", i)
				job.Videos = append(job.Videos, model.Video{ID: id, Label: id, Encoded: seed, DAR: 4.0 / 3})
				disc.VideoIDs = append(disc.VideoIDs, id)
			}
			log, err := os.Create(filepath.Join(root, fmt.Sprintf("author-%d.log", fixture.titles)))
			if err != nil {
				t.Fatal(err)
			}
			defer log.Close()
			iso, err := tools.Author(ctx, job, disc, 1, dir, log, func(string) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			for _, title := range []int{1, fixture.titles} {
				if err := process.Run(ctx, tools.FFprobe, []string{"-v", "error", "-f", "dvdvideo", "-title", fmt.Sprint(title), "-i", iso, "-show_streams"}, process.Options{}); err != nil {
					t.Fatalf("DVD title %d: %v", title, err)
				}
			}
		})
	}
}
