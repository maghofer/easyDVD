package media

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"easydvd/internal/menu"
	"easydvd/internal/model"
	"easydvd/internal/process"
)

func AuthorXML(videos []model.Video, pages []menu.Page, s model.Settings, dir string) string {
	var b strings.Builder
	b.WriteString("<dvdauthor dest=\"dvd\">\n<vmgm><fpc>g0 = 0; g3 = 1024; jump vmgm menu 1;</fpc><menus>\n")
	aspect := s.Menu.Defaults().Aspect
	wide := ""
	if aspect == "16:9" {
		wide = " widescreen=\"nopanscan\""
	}
	fmt.Fprintf(&b, "<video format=\"%s\" aspect=\"%s\"%s/><audio format=\"ac3\" channels=\"2\" samplerate=\"48khz\"/>\n", strings.ToLower(s.Standard), aspect, wide)
	if aspect == "16:9" {
		b.WriteString("<subpicture><stream mode=\"widescreen\" id=\"0\"/><stream mode=\"letterbox\" id=\"1\"/></subpicture>\n")
	}
	for _, p := range pages {
		entry := ""
		if p.Number == 1 {
			entry = " entry=\"title\""
		}
		fmt.Fprintf(&b, "<pgc%s><pre>", entry)
		// A Play All transition enters the next title's page with g1 set to
		// that disc-global title number. Dispatch only this page's titles:
		// the command count stays bounded by TitlesPerPage, even on 99-title
		// discs. Direct VMGM JumpTT commands need no generated jump pads.
		b.WriteString("if (g0 eq 1) { ")
		for i := (p.Number - 1) * s.TitlesPerPage; i < min(p.Number*s.TitlesPerPage, len(videos)); i++ {
			fmt.Fprintf(&b, "if (g1 eq %d) { jump title %d; } ", i+1, i+1)
		}
		fmt.Fprintf(&b, "} g0 = 0; button = g3;</pre><vob file=\"page-%03d.mpg\" pause=\"inf\"/>\n", p.Number)
		for _, bt := range p.Buttons {
			fmt.Fprintf(&b, "<button name=\"%s\">%s</button>\n", bt.Name, menu.Escape(bt.Command))
		}
		b.WriteString("</pgc>\n")
	}
	b.WriteString("</menus></vmgm>\n")
	for i, v := range videos {
		page := i/s.TitlesPerPage + 1
		relative, _ := filepath.Rel(dir, v.Encoded)
		relative = filepath.ToSlash(relative)
		fmt.Fprintf(&b, "<titleset><menus><video format=\"%s\" aspect=\"%s\"%s/><pgc entry=\"root\"><pre>g0 = 0; jump vmgm menu %d;</pre></pgc></menus>\n", strings.ToLower(s.Standard), aspect, wide, page)
		fmt.Fprintf(&b, "<titles><video format=\"%s\" aspect=\"%s\"/><audio format=\"ac3\" channels=\"2\" samplerate=\"48khz\"/><pgc><vob file=\"%s\"/><post>", strings.ToLower(s.Standard), v.Aspect(), menu.Escape(relative))
		if i+1 < len(videos) {
			fmt.Fprintf(&b, "if (g0 eq 1) { g1 = %d; call vmgm menu %d; } ", i+2, (i+1)/s.TitlesPerPage+1)
		} else {
			b.WriteString("if (g0 eq 1) { g0 = 0; g3 = 1024; call vmgm menu 1; } ")
		}
		fmt.Fprintf(&b, "call vmgm menu %d;</post></pgc></titles></titleset>\n", page)
	}
	b.WriteString("</dvdauthor>\n")
	return b.String()
}

func (t Tools) Author(ctx context.Context, j model.Job, disc model.Disc, total int, dir string, log io.Writer, stage func(string) error) (string, error) {
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
			return "", fmt.Errorf("disc references unknown video %s", id)
		}
	}
	if len(videos) == 0 {
		return "", fmt.Errorf("cannot author an empty disc")
	}
	if err := stage("Creating menus"); err != nil {
		return "", err
	}
	pages, err := menu.Render(dir, j.Title, disc.Number, total, videos, j.Settings)
	if err != nil {
		return "", err
	}
	for _, p := range pages {
		if err := stage(fmt.Sprintf("Creating menu page %d of %d", p.Number, len(pages))); err != nil {
			return "", err
		}
		base := fmt.Sprintf("page-%03d-base.mpg", p.Number)
		final := fmt.Sprintf("page-%03d.mpg", p.Number)
		target := strings.ToLower(j.Settings.Standard) + "-dvd"
		args := []string{"-hide_banner", "-nostdin", "-y", "-loop", "1", "-i", filepath.Base(p.Background), "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo", "-target", target, "-aspect", j.Settings.Menu.Defaults().Aspect, "-b:v", "2000k", "-b:a", "256k", "-ac", "2", "-t", "1", base}
		if err = process.Run(ctx, t.FFmpeg, args, process.Options{Dir: dir, Log: log}); err != nil {
			return "", err
		}
		in, e := os.Open(filepath.Join(dir, base))
		if e != nil {
			return "", e
		}
		out, e := os.Create(filepath.Join(dir, final))
		if e != nil {
			in.Close()
			return "", e
		}
		err = process.Run(ctx, t.Spumux, []string{"-s", "0", filepath.Base(p.XML)}, process.Options{Dir: dir, Stdin: in, Stdout: out, Log: log, Env: []string{"VIDEO_FORMAT=" + strings.ToUpper(j.Settings.Standard)}})
		in.Close()
		ce := out.Close()
		if err != nil {
			return "", err
		}
		if ce != nil {
			return "", ce
		}
		if p.LetterboxXML != "" {
			wideFile := filepath.Join(dir, fmt.Sprintf("page-%03d-wide.mpg", p.Number))
			if err = os.Rename(filepath.Join(dir, final), wideFile); err != nil {
				return "", err
			}
			in, err = os.Open(wideFile)
			if err != nil {
				return "", err
			}
			out, err = os.Create(filepath.Join(dir, final))
			if err != nil {
				in.Close()
				return "", err
			}
			err = process.Run(ctx, t.Spumux, []string{"-s", "1", filepath.Base(p.LetterboxXML)}, process.Options{Dir: dir, Stdin: in, Stdout: out, Log: log, Env: []string{"VIDEO_FORMAT=" + strings.ToUpper(j.Settings.Standard)}})
			in.Close()
			ce = out.Close()
			if err != nil {
				return "", err
			}
			if ce != nil {
				return "", ce
			}
		}
	}
	xml := AuthorXML(videos, pages, j.Settings, dir)
	if err = os.WriteFile(filepath.Join(dir, "dvd.xml"), []byte(xml), 0644); err != nil {
		return "", err
	}
	// dir is always a private generation directory owned by this job.
	if err = os.RemoveAll(filepath.Join(dir, "dvd")); err != nil {
		return "", err
	}
	if err := stage("Authoring DVD"); err != nil {
		return "", err
	}
	if err = process.Run(ctx, t.DVDAuthor, []string{"-x", "dvd.xml"}, process.Options{Dir: dir, Log: log, Env: []string{"VIDEO_FORMAT=" + j.Settings.Standard}}); err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Join(dir, "dvd", "AUDIO_TS"), 0755); err != nil {
		return "", err
	}
	if err = ValidateStructure(filepath.Join(dir, "dvd"), len(videos)); err != nil {
		return "", err
	}
	if err := stage("Creating ISO"); err != nil {
		return "", err
	}
	volume := fmt.Sprintf("EASYDVD_%03d", disc.Number)
	if err = process.Run(ctx, t.Mkisofs, []string{"-dvd-video", "-udf", "-V", volume, "-o", "disc.iso", "dvd"}, process.Options{Dir: dir, Log: log}); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "disc.iso")
	st, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if st.Size() < 32768 {
		return "", fmt.Errorf("ISO is unexpectedly small")
	}
	return path, nil
}

func ValidateStructure(dir string, titles int) error {
	for _, name := range []string{"VIDEO_TS.IFO", "VIDEO_TS.BUP"} {
		st, e := os.Stat(filepath.Join(dir, "VIDEO_TS", name))
		if e != nil || st.Size() == 0 {
			return fmt.Errorf("DVD structure missing %s", name)
		}
	}
	for i := 1; i <= titles; i++ {
		for _, ext := range []string{"0.IFO", "0.BUP", "1.VOB"} {
			name := fmt.Sprintf("VTS_%02d_%s", i, ext)
			st, e := os.Stat(filepath.Join(dir, "VIDEO_TS", name))
			if e != nil || st.Size() == 0 {
				return fmt.Errorf("DVD structure missing %s", name)
			}
		}
	}
	return nil
}
