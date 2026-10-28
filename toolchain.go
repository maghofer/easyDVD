// Package easydvd shares pinned distribution metadata between the app and build scripts.
package easydvd

import (
	"embed"
	"encoding/json"
)

//go:embed scripts/tools.lock.json
var metadata embed.FS

type FFmpegRelease struct {
	Version          string `json:"version"`
	URL              string `json:"url"`
	SHA256           string `json:"sha256"`
	Archive          string `json:"archive"`
	FFmpegExeSHA256  string `json:"ffmpegExeSHA256"`
	FFprobeExeSHA256 string `json:"ffprobeExeSHA256"`
}

func RecommendedFFmpeg() FFmpegRelease {
	b, err := metadata.ReadFile("scripts/tools.lock.json")
	if err != nil {
		panic(err)
	}
	var pins struct {
		FFmpeg FFmpegRelease `json:"ffmpeg"`
	}
	if err = json.Unmarshal(b, &pins); err != nil {
		panic(err)
	}
	return pins.FFmpeg
}
