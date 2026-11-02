package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"easydvd/internal/model"
)

// Optional throughput comparison uses four titles from the same staged source.
// It includes source decoding, production encoding, decode validation and hashing.
func TestBlurayParallelThroughput(t *testing.T) {
	input := os.Getenv("EASYDVD_BLURAY_BENCH_INPUT")
	if input == "" {
		t.Skip("set EASYDVD_BLURAY_BENCH_INPUT to a short staged HD source")
	}
	tools, ctx := blurayTools(t)
	v, err := tools.Probe(ctx, input, filepath.Base(input))
	if err != nil {
		t.Fatal(err)
	}
	v, err = tools.DetectPillarbox(ctx, v, input, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		Profile     string
		Workers     int
		LogicalCPUs int
		Seconds     float64
	}
	var results []result
	for _, profile := range []string{model.ProfileBalanced, model.ProfileQuality} {
		for _, workers := range []int{2, 4} {
			dir := t.TempDir()
			s := model.DefaultSettings(dir)
			s.DiscFormat = "Blu-ray"
			s.Media = "BD-25"
			s.EncodingProfile = profile
			s.Encoders = workers
			tasks := make(chan int, 4)
			for i := 0; i < 4; i++ {
				tasks <- i
			}
			close(tasks)
			var wg sync.WaitGroup
			errs := make(chan error, 4)
			start := time.Now()
			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for n := range tasks {
						sample := v
						sample.ID = fmt.Sprint(n)
						_, err := tools.Encode(ctx, sample, s, input, filepath.Join(dir, sample.ID), io.Discard, nil)
						if err != nil {
							errs <- err
						}
					}
				}()
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				t.Fatal(err)
			}
			r := result{profile, workers, runtime.NumCPU(), time.Since(start).Seconds()}
			results = append(results, r)
			t.Logf("%s workers=%d: %.3fs for four titles", profile, workers, r.Seconds)
		}
	}
	if path := os.Getenv("EASYDVD_BLURAY_BENCH_REPORT"); path != "" {
		b, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
