package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeCapabilities(ff, probe, missing string) commandOutput {
	return func(ctx context.Context, exe string, args []string) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		switch args[len(args)-1] {
		case "-version":
			if strings.Contains(exe, "probe") {
				return "ffprobe version " + probe + "-build\n", nil
			}
			return "ffmpeg version " + ff + "-build\n", nil
		case "-encoders":
			return " V.S..D mpeg2video MPEG-2\n A....D ac3 AC3\n V....D mjpeg MJPEG\n V....D ffv1 FFV1\n", nil
		case "-decoders":
			return " V..... mpeg2video MPEG-2\n A..... ac3 AC3\n V..... png PNG\n", nil
		case "-filters":
			names := []string{"scale", "pad", "setsar", "fps", "bwdif", "apad", "anullsrc", "crop", "bbox", "format"}
			out := ""
			for _, name := range names {
				if name != missing {
					out += " .. " + name + " filter\n"
				}
			}
			return out, nil
		case "-muxers":
			return " E dvd DVD\n E null NULL\n E image2 images\n E matroska Matroska\n", nil
		}
		return "", fmt.Errorf("unexpected command")
	}
}
func TestFFmpegCompatibility(t *testing.T) {
	pair := FFmpegPair{FFmpeg: "ffmpeg", FFprobe: "ffprobe"}
	for _, test := range []struct {
		ff, probe, missing string
		ok                 bool
	}{{"9.0.2", "9.0.2", "", true}, {"9.1", "9.1", "", true}, {"8.1", "8.1", "", false}, {"9.0.1", "9.0.1", "", false}, {"10.0", "10.0", "", false}, {"N-123", "N-123", "", false}, {"9.0.2", "9.1", "", false}, {"9.0.2", "9.0.2", "bwdif", false}} {
		t.Run(test.ff+"/"+test.probe+"/"+test.missing, func(t *testing.T) {
			got, err := validateFFmpeg(context.Background(), pair, fakeCapabilities(test.ff, test.probe, test.missing))
			if (err == nil) != test.ok {
				t.Fatal(got, err)
			}
			if err == nil && got.Version == "" {
				t.Fatal("missing release version")
			}
		})
	}
}
func TestResolveSkipsIncompatiblePair(t *testing.T) {
	bad := pairAt(filepath.Join(t.TempDir(), "old"))
	good := pairAt(filepath.Join(t.TempDir(), "supported"))
	for _, p := range []FFmpegPair{bad, good} {
		for _, file := range []string{p.FFmpeg, p.FFprobe} {
			os.MkdirAll(filepath.Dir(file), 0755)
			os.WriteFile(file, []byte("fixture"), 0644)
		}
	}
	calls := 0
	got, err := resolveCandidates(context.Background(), Tools{DVDAuthor: "bundled-dvdauthor"}, []FFmpegPair{bad, bad, good}, func(ctx context.Context, p FFmpegPair) (FFmpegPair, error) {
		calls++
		if p.FFmpeg == bad.FFmpeg {
			return p, fmt.Errorf("old release")
		}
		p.Version = "9.0.2"
		return p, nil
	})
	if err != nil || got.FFmpeg != good.FFmpeg || got.DVDAuthor != "bundled-dvdauthor" || calls != 2 {
		t.Fatal(got, err, calls)
	}
}
func TestMissingToolsExplainSetup(t *testing.T) {
	tools, err := resolveCandidates(context.Background(), Tools{}, []FFmpegPair{pairAt(t.TempDir())}, ValidateFFmpeg)
	if err == nil || tools.FFmpeg != "" || !strings.Contains(tools.Require().Error(), "setup required") {
		t.Fatal(tools, err)
	}
}
func TestSaveFFmpegChoice(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	t.Setenv("XDG_CONFIG_HOME", root)
	choice := filepath.Join(root, "external", "bin")
	if err := SaveFFmpegFolder(choice); err != nil {
		t.Fatal(err)
	}
	if savedFFmpegFolder() != choice {
		t.Fatal("location not saved")
	}
	second := filepath.Join(root, "other")
	if err := SaveFFmpegFolder(second); err != nil {
		t.Fatal(err)
	}
	if savedFFmpegFolder() != second {
		t.Fatal("location not replaced")
	}
	if SaveFFmpegFolder("relative/path") == nil {
		t.Fatal("relative configuration accepted")
	}
}

func TestDiscoveryDoesNotExecuteCurrentDirectoryTools(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	for _, name := range []string{"APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_CACHE_HOME"} {
		t.Setenv(name, root)
	}
	t.Setenv("EASYDVD_TOOLS", "")
	t.Setenv("EASYDVD_FFMPEG", "")
	t.Setenv("PATH", ".")
	t.Setenv("GODEBUG", "execerrdot=1")
	pair := pairAt(root)
	for _, path := range []string{pair.FFmpeg, pair.FFprobe} {
		if err := os.WriteFile(path, []byte("not an executable"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	tools, err := ResolveTools(context.Background(), "")
	if err == nil || tools.FFmpeg != "" {
		t.Fatal("current-directory tools accepted")
	}
	if strings.Contains(err.Error(), pair.FFmpeg) {
		t.Fatal("attempted to execute a stray current-directory binary", err)
	}
}
