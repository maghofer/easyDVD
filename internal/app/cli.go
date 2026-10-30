package app

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"easydvd"
	"easydvd/internal/engine"
	"easydvd/internal/fsops"
	"easydvd/internal/media"
	"easydvd/internal/model"
	"easydvd/internal/store"
)

func Paths() (string, string) {
	base, _ := os.UserConfigDir()
	home, _ := os.UserHomeDir()
	return filepath.Join(base, "easyDVD"), filepath.Join(home, "Documents")
}

// UserCacheDir maps to LocalAppData on Windows; logs are kept off roaming profiles.
func LocalDataDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "easyDVD"), nil
}

func Worker(args []string) (bool, error) {
	if len(args) > 0 && args[0] == "--scan-worker" {
		if len(args) < 2 {
			return true, fmt.Errorf("scan worker needs a folder")
		}
		return true, media.ScanWorker(args[1], args[2:], os.Stdout)
	}
	if len(args) > 0 && args[0] == "--fs-worker" {
		return true, fsops.Worker(args[1:], os.Stdout)
	}
	return false, nil
}

func CLI(args []string) error {
	if ok, err := Worker(args); ok {
		return err
	}
	config, base := Paths()
	defaults := model.DefaultSettings(base)
	fs := flag.NewFlagSet("easyDVD", flag.ContinueOnError)
	showVersion := fs.Bool("version", false, "Print easyDVD version and exit")
	output := fs.String("output", defaults.OutputDir, "Output directory")
	work := fs.String("work", defaults.WorkDir, "Local work directory")
	state := fs.String("state", filepath.Join(config, "queue.db"), "SQLite queue path")
	standard := fs.String("standard", "PAL", "PAL or NTSC")
	medium := fs.String("media", "DVD-5", "DVD-5 or DVD-9")
	menuAspect := fs.String("menu-aspect", "4:3", "Menu aspect: 4:3 or 16:9")
	efficient := fs.Bool("efficient", false, "Optimize disc packing")
	direct := fs.Bool("direct", false, "Read original sources directly")
	keep := fs.Bool("keep-work", false, "Retain intermediate media")
	thumbs := fs.Bool("thumbnails", false, "Thumbnail menus")
	noMenu := fs.Bool("no-menu", false, "Play all included videos automatically in order without a menu")
	combine := fs.Bool("combine-files", false, "Combine explicit video files into one DVD set")
	perPage := fs.Int("titles", 10, "Titles per page (1–12 text, 1–6 thumbnails)")
	tools := fs.Bool("check-tools", false, "Print tool diagnostics and exit")
	authoringOnly := fs.Bool("check-authoring-tools", false, "Check bundled authoring tools without requiring FFmpeg")
	ffmpegDir := fs.String("ffmpeg-dir", "", "Use FFmpeg and FFprobe from this folder for this run")
	downloadFFmpeg := fs.Bool("download-ffmpeg", false, "Download and remember the recommended private FFmpeg installation")
	resume := fs.Bool("resume", false, "Resume saved queue")
	workers := fs.Int("encoders", 2, "Parallel encoders (1–2)")
	profile := fs.String("profile", model.ProfileBalanced, "Encoding profile: Balanced or Quality")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Println("easyDVD " + easydvd.Version())
		return nil
	}
	t := media.FindAuthoringTools()
	if *authoringOnly {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		b, _ := json.MarshalIndent(t.CheckAuthoring(ctx), "", "  ")
		fmt.Println(string(b))
		return t.RequireAuthoring()
	}
	setupCtx, stopSetup := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSetup()
	if *downloadFFmpeg {
		last := ""
		pair, err := media.InstallRecommendedFFmpeg(setupCtx, func(p media.DownloadProgress) {
			message := p.Stage
			if p.Total > 0 {
				message += fmt.Sprintf(" · %.0f%%", float64(p.Downloaded)*100/float64(p.Total))
			}
			if message != last {
				fmt.Fprintln(os.Stderr, message)
				last = message
			}
		})
		if err != nil {
			return err
		}
		if err = media.SaveFFmpegFolder(filepath.Dir(pair.FFmpeg)); err != nil {
			return err
		}
		t.FFmpeg = pair.FFmpeg
		t.FFprobe = pair.FFprobe
	} else {
		ctx, cancel := context.WithTimeout(setupCtx, 30*time.Second)
		resolved, err := media.ResolveTools(ctx, strings.TrimSpace(*ffmpegDir))
		cancel()
		t = resolved
		if err != nil && !*tools {
			return fmt.Errorf("%w; use -ffmpeg-dir or -download-ffmpeg", err)
		}
	}
	if *tools {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		b, _ := json.MarshalIndent(t.Check(ctx), "", "  ")
		fmt.Println(string(b))
		if err := t.Require(); err != nil {
			return err
		}
		return nil
	}
	if *downloadFFmpeg && fs.NArg() == 0 && !*resume {
		return t.Require()
	}
	if err := t.Require(); err != nil {
		return err
	}
	if fs.NArg() == 0 && !*resume {
		fs.Usage()
		return fmt.Errorf("add at least one source folder or use -resume")
	}
	defaults.OutputDir = *output
	defaults.WorkDir = *work
	defaults.Standard = *standard
	defaults.Menu.Aspect = *menuAspect
	defaults.Media = *medium
	defaults.StageLocally = !*direct
	defaults.KeepWork = *keep
	defaults.NoMenu = *noMenu
	defaults.Encoders = *workers
	defaults.EncodingProfile = *profile
	defaults.TitlesPerPage = *perPage
	if *efficient {
		defaults.Packing = "Space efficiency"
	}
	if *thumbs {
		defaults.Layout = "Thumbnails"
		if *perPage == 10 {
			defaults.TitlesPerPage = 6
		}
	}
	if err := defaults.Validate(); err != nil {
		return err
	}
	st, err := store.Open(*state)
	if err != nil {
		return err
	}
	defer st.Close()
	var options []engine.Option
	if filepath.Clean(*state) == filepath.Join(config, "queue.db") {
		dataDir, pathErr := LocalDataDir()
		if pathErr != nil {
			return pathErr
		}
		options = append(options, engine.WithDataDir(dataDir))
	}
	e, err := engine.New(st, defaults, t, options...)
	if err != nil {
		return err
	}
	defer e.Close()
	if !*resume {
		if err = e.SetSettings("", defaults); err != nil {
			return err
		}
	} else {
		for _, j := range e.Snapshot().Jobs {
			if j.State == model.Blocked || j.State == model.Canceled {
				e.Retry(j.ID)
			}
		}
	}
	if *combine {
		err = e.AddFiles(fs.Args())
		if err != nil {
			return err
		}
	} else {
		for _, folder := range fs.Args() {
			if err = e.Add(folder); err != nil {
				return err
			}
		}
	}
	e.Start()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	previous := ""
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			s := e.Snapshot()
			if s.Error != "" {
				return fmt.Errorf("%s", s.Error)
			}
			pending := false
			blocked := false
			message := ""
			for _, j := range s.Jobs {
				message += fmt.Sprintf("%s: %s · %s · %.0f%%\n", j.Title, j.State, j.Stage, j.Progress*100)
				if j.State == model.Ready || j.State == model.Scanning || j.State == model.Running {
					pending = true
				}
				if j.State == model.Blocked {
					blocked = true
					message += j.Error + "\n"
				}
			}
			if message != previous {
				fmt.Print(message)
				previous = message
			}
			if !pending {
				if blocked {
					return fmt.Errorf("one or more sets need attention; completed work has been saved")
				}
				return nil
			}
		}
	}
}
