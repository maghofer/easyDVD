package media

import (
	"runtime"
	"strconv"

	"easydvd/internal/model"
)

func encodingThreads(s model.Settings) int {
	limit := 8
	// HD AVC benefits from more parallelism within a lone title. Keep the
	// original MPEG-2 budget and divide HD resources across concurrent titles.
	if s.IsBluray() {
		limit = 16
	}
	return min(limit, max(1, (runtime.NumCPU()-2)/s.EncoderWorkers(runtime.NumCPU())))
}

// Each FFmpeg process owns separate decoder, filter and encoder pools. Bound
// all three when running many titles; their defaults otherwise each see the
// entire machine. This does not change the video filters or quality options.
func ffmpegInputArgs(s model.Settings) []string {
	threads := strconv.Itoa(encodingThreads(s))
	return []string{"-hide_banner", "-nostdin", "-y", "-xerror", "-filter_threads", threads, "-threads:v", threads, "-err_detect", "explode"}
}
