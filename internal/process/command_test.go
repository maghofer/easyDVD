package process

import (
	"runtime"
	"strings"
	"testing"
)

func TestDiagnosticCommandsPreserveLiteralArguments(t *testing.T) {
	got := CommandLine(`C:\Media tools\ffmpeg.exe`, []string{"-i", `C:\Videos\O'Brien $clip.mov`, "-vf", "scale=720:576,pad=720:576:(ow-iw)/2:0"})
	if runtime.GOOS == "windows" {
		if !strings.HasPrefix(got, `& 'C:\Media tools\ffmpeg.exe' -i 'C:\Videos\O''Brien $clip.mov'`) {
			t.Fatal(got)
		}
	} else if !strings.Contains(got, `O'\''Brien`) {
		t.Fatal(got)
	}
	if !strings.Contains(got, `'scale=720:576,pad=720:576:(ow-iw)/2:0'`) {
		t.Fatal("filter not quoted", got)
	}
}
