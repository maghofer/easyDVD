package media

import (
	"runtime"
	"strconv"
	"testing"

	"easydvd/internal/model"
)

func TestFFmpegThreadBudgets(t *testing.T) {
	s := model.DefaultSettings(t.TempDir())
	v := model.Video{Width: 720, Height: 576, Duration: 4, FPS: 25, DAR: 4.0 / 3, AudioStream: -1}
	for _, workers := range []int{0, 1, 2, model.MaxEncoders} {
		s.Encoders = workers
		want := min(8, max(1, (runtime.NumCPU()-2)/s.EncoderWorkers(runtime.NumCPU())))
		for _, args := range [][]string{EncodeArgs(v, s, "in", "pass", "out", 2), HCencPrepareArgs(v, s, "in", "out")} {
			input := -1
			decoder, encoder, filter := "", "", ""
			for i := 0; i < len(args)-1; i++ {
				switch args[i] {
				case "-i":
					if input < 0 {
						input = i
					}
				case "-threads:v", "-threads":
					if input < 0 {
						decoder = args[i+1]
					} else {
						encoder = args[i+1]
					}
				case "-filter_threads":
					filter = args[i+1]
				}
			}
			budget := strconv.Itoa(want)
			if decoder != budget || encoder != budget || filter != budget {
				t.Fatal("decoder/filter/encoder thread pools not bounded", workers, args)
			}
		}
	}
}

func TestBlurayThreadBudgetSharesCPUAndAllowsMoreThanDVD(t *testing.T) {
	s := model.DefaultSettings(t.TempDir())
	s.DiscFormat = "Blu-ray"
	for _, workers := range []int{1, 2, 4, model.MaxEncoders} {
		s.Encoders = workers
		want := min(16, max(1, (runtime.NumCPU()-2)/workers))
		if encodingThreads(s) != want {
			t.Fatal("wrong HD thread budget", workers, encodingThreads(s), want)
		}
	}
}
