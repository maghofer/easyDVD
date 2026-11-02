package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"easydvd/internal/menu"
	"easydvd/internal/model"
	"easydvd/internal/process"
)

const blurayRuntimeID = "experimental-2026.11.1"

func findBlurayRuntime() string {
	if root := os.Getenv("EASYDVD_BLURAY"); root != "" {
		absolute, _ := filepath.Abs(root)
		return absolute
	}
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	for _, root := range []string{filepath.Join(filepath.Dir(exe), "tools", "bluray"), filepath.Join(cwd, ".tools", "media", "bin", "bluray")} {
		if firstFile(filepath.Join(root, "python", "python.exe")) != "" {
			return root
		}
	}
	return ""
}

func (t Tools) CheckBluray(ctx context.Context) ToolStatus {
	s := ToolStatus{Name: "Blu-ray authoring (experimental)", Path: t.BlurayRoot}
	if t.BlurayRoot == "" {
		s.Error = "Bundled tools/bluray runtime is missing"
		return s
	}
	var output bytes.Buffer
	err := process.Run(ctx, filepath.Join(t.BlurayRoot, "python", "python.exe"), []string{filepath.Join(t.BlurayRoot, "author.py"), "--check"}, process.Options{Stdout: &output})
	if err != nil {
		s.Error = err.Error()
		return s
	}
	var check struct {
		Stage    string `json:"stage"`
		Protocol int    `json:"protocol"`
		Runtime  string `json:"runtime"`
	}
	if err = json.Unmarshal(bytes.TrimSpace(output.Bytes()), &check); err != nil || check.Stage != "ready" || check.Protocol != 1 || check.Runtime != blurayRuntimeID {
		s.Error = "Incompatible Blu-ray runtime"
		return s
	}
	s.Version = check.Runtime
	return s
}

func (t Tools) RequireFor(ctx context.Context, s model.Settings) error {
	if err := t.RequireMedia(); err != nil {
		return err
	}
	if !s.IsBluray() {
		if err := t.RequireAuthoring(); err != nil {
			return err
		}
		return t.RequireProfile(s)
	}
	if check := t.CheckBluray(ctx); check.Error != "" {
		return fmt.Errorf("Blu-ray runtime: %s; restore tools/bluray or set EASYDVD_BLURAY", check.Error)
	}
	for _, check := range []struct {
		arg   string
		names []string
	}{
		{"-encoders", []string{"libx264", "ac3"}}, {"-filters", []string{"scale", "pad", "fps", "setsar", "bwdif", "tinterlace", "fieldorder", "apad", "anullsrc", "colorspace"}},
		{"-muxers", []string{"mpegts"}}, {"-bsfs", []string{"h264_mp4toannexb", "trace_headers"}},
	} {
		out, err := toolOutput(ctx, t.FFmpeg, []string{"-hide_banner", check.arg})
		if err != nil {
			return err
		}
		for _, name := range check.names {
			found := false
			for _, word := range strings.Fields(out) {
				if word == name {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("FFmpeg lacks Blu-ray capability %s", name)
			}
		}
	}
	return nil
}

type blurayManifest struct {
	Version   int              `json:"version"`
	FFmpeg    string           `json:"ffmpeg"`
	Directory string           `json:"directory"`
	ISO       string           `json:"iso"`
	Label     string           `json:"label"`
	Titles    []blurayTitle    `json:"titles"`
	Menus     []blurayPage     `json:"menus"`
	Playlists []blurayPlaylist `json:"playlists"`
}
type blurayTitle struct {
	Clip     string `json:"clip"`
	FPS      [2]int `json:"fps"`
	Video    string `json:"video"`
	Aspect   int    `json:"aspect"`
	Channels int    `json:"channels"`
	End      string `json:"end"`
}
type blurayPage struct {
	ID         string         `json:"id"`
	Background string         `json:"background"`
	Buttons    []blurayButton `json:"buttons"`
}
type blurayButton struct {
	ID        string            `json:"id"`
	X         int               `json:"x"`
	Y         int               `json:"y"`
	Selected  string            `json:"selected"`
	Activated string            `json:"activated"`
	Action    string            `json:"action"`
	Nav       map[string]string `json:"nav"`
}
type blurayPlaylist struct {
	ID     string `json:"id"`
	Titles []int  `json:"titles"`
	End    string `json:"end"`
}

func (t Tools) authorBluray(ctx context.Context, j model.Job, disc model.Disc, total int, dir string, log io.Writer, stage func(string) error) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	var videos []model.Video
	for _, id := range disc.VideoIDs {
		found := false
		for _, v := range j.Videos {
			if v.ID == id {
				videos = append(videos, v)
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("unknown Blu-ray title %s", id)
		}
	}
	if len(videos) == 0 {
		return "", fmt.Errorf("cannot author an empty Blu-ray")
	}
	manifest := blurayManifest{Version: 1, FFmpeg: t.FFmpeg, Directory: filepath.Join(dir, "bluray"), ISO: filepath.Join(dir, "disc.iso"), Label: fmt.Sprintf("EASYDVD_BD_%03d", disc.Number)}
	var all []int
	for n, v := range videos {
		p := v.Bluray
		if p.Version != 1 || v.Encoded == "" {
			return "", fmt.Errorf("title %s has no prepared Blu-ray media", v.Label)
		}
		end := "stop"
		if !j.Settings.NoMenu {
			end = fmt.Sprintf("menu:page%d:title%03d", n/j.Settings.TitlesPerPage+1, n+1)
		} else if n+1 < len(videos) {
			end = fmt.Sprintf("title:%d", n+2)
		}
		manifest.Titles = append(manifest.Titles, blurayTitle{v.Encoded, [2]int{p.FPSNum, p.FPSDen}, p.Mode, p.Aspect, p.Channels, end})
		all = append(all, n+1)
	}
	if !j.Settings.NoMenu {
		if err := stage("Creating Blu-ray menus"); err != nil {
			return "", err
		}
		pages, err := menu.Render(dir, j.Title, disc.Number, total, videos, j.Settings)
		if err != nil {
			return "", err
		}
		for _, p := range pages {
			page := blurayPage{ID: fmt.Sprintf("page%d", p.Number), Background: p.Background}
			for k, b := range p.Buttons {
				action := ""
				switch b.Action.Kind {
				case "title":
					action = fmt.Sprintf("title:%d", b.Action.Target)
				case "page":
					action = fmt.Sprintf("menu:page%d", b.Action.Target)
				case "playall":
					action = "playlist:all"
				default:
					return "", fmt.Errorf("unsupported Blu-ray button action")
				}
				prev, next := p.Buttons[(k+len(p.Buttons)-1)%len(p.Buttons)].Name, p.Buttons[(k+1)%len(p.Buttons)].Name
				page.Buttons = append(page.Buttons, blurayButton{b.Name, b.Rect.Min.X, b.Rect.Min.Y, b.Selected, b.Activated, action, map[string]string{"up": prev, "left": prev, "down": next, "right": next}})
			}
			manifest.Menus = append(manifest.Menus, page)
		}
		manifest.Playlists = []blurayPlaylist{{"all", all, "menu:page1"}}
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "bluray.json")
	if err = os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	// This fixed child is owned by this job's private generation directory.
	if err = os.RemoveAll(manifest.Directory); err != nil {
		return "", err
	}
	if err = stage("Authoring Blu-ray"); err != nil {
		return "", err
	}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	var stageErr error
	complete := false
	err = process.Run(child, filepath.Join(t.BlurayRoot, "python", "python.exe"), []string{filepath.Join(t.BlurayRoot, "author.py"), path}, process.Options{Dir: dir, Log: log, Env: []string{"TEMP=" + dir, "TMP=" + dir}, Line: func(line string) {
		var event struct {
			Stage string `json:"stage"`
		}
		if json.Unmarshal([]byte(line), &event) != nil {
			return
		}
		if event.Stage == "complete" {
			complete = true
		}
		if event.Stage == "iso" && stageErr == nil {
			stageErr = stage("Creating Blu-ray ISO")
			if stageErr != nil {
				cancel()
			}
		}
	}})
	if stageErr != nil {
		return "", stageErr
	}
	if err != nil {
		return "", err
	}
	if !complete {
		return "", fmt.Errorf("Blu-ray helper did not finish")
	}
	if err = validateBlurayStructure(manifest.Directory, len(videos), len(manifest.Menus)); err != nil {
		return "", err
	}
	f, err := os.Open(manifest.ISO)
	if err != nil {
		return "", err
	}
	defer f.Close()
	header := make([]byte, 65536)
	n, err := f.Read(header)
	if err != nil {
		return "", err
	}
	if !bytes.Contains(header[:n], []byte("NSR03")) {
		return "", fmt.Errorf("Blu-ray ISO lacks UDF volume recognition")
	}
	return manifest.ISO, nil
}

func validateBlurayStructure(dir string, titles, menus int) error {
	for _, name := range []string{"index.bdmv", "MovieObject.bdmv"} {
		a, err := os.ReadFile(filepath.Join(dir, "BDMV", name))
		if err != nil || len(a) < 40 {
			return fmt.Errorf("Blu-ray structure missing %s", name)
		}
		b, err := os.ReadFile(filepath.Join(dir, "BDMV", "BACKUP", name))
		if err != nil || !bytes.Equal(a, b) {
			return fmt.Errorf("Blu-ray backup mismatch: %s", name)
		}
	}
	for n := 1; n <= titles+menus; n++ {
		for _, part := range []struct{ folder, ext string }{{"CLIPINF", "clpi"}, {"STREAM", "m2ts"}} {
			path := filepath.Join(dir, "BDMV", part.folder, fmt.Sprintf("%05d.%s", n, part.ext))
			st, err := os.Stat(path)
			if err != nil || st.Size() == 0 {
				return fmt.Errorf("Blu-ray structure missing %s", filepath.Base(path))
			}
		}
	}
	return nil
}
