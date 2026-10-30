package ui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
)

func (u *window) jobDetails() {
	selected := u.selectedJob()
	if selected == nil {
		return
	}
	j := selected.Clone()
	var text strings.Builder
	fmt.Fprintf(&text, "%s\n%s\n\nState: %s\nStage: %s\nSettings locked: %v\nFormat: %s / %s\nPacking: %s\n\n%s\n", j.Title, j.Folder, j.State, j.Stage, j.Locked, j.Settings.Standard, j.Settings.Media, j.Settings.Packing, j.Error)
	for _, d := range j.Discs {
		fmt.Fprintf(&text, "\nDVD %d: %d titles · %.2f GB\n%s\nSHA-256: %s\n", d.Number, len(d.VideoIDs), float64(d.ISOBytes)/1e9, d.ISO, d.SHA256)
		for _, id := range d.VideoIDs {
			for _, v := range j.Videos {
				if v.ID == id {
					fmt.Fprintf(&text, "  %s\n    %s\n", v.Label, v.Relative)
					break
				}
			}
		}
	}
	for _, v := range j.Videos {
		if v.Error != "" {
			fmt.Fprintf(&text, "\n%s:\n%s\n", v.Relative, v.Error)
		}
	}
	_, summary := newReadOnlyText(text.String())
	logText, logView := newReadOnlyText("Loading job log…")
	tabs := container.NewAppTabs(container.NewTabItem("Job and disc assignments", container.NewScroll(summary)), container.NewTabItem("Log (last 64 KB)", container.NewScroll(logView)))
	d := dialog.NewCustom("Job details", "Close", tabs, u.win)
	d.Resize(fyne.NewSize(850, 620))
	d.Show()
	u.action(func() error {
		path := u.engine.JobLogPath(j.ID)
		f, err := os.Open(path)
		if os.IsNotExist(err) {
			f, err = os.Open(filepath.Join(j.Settings.WorkDir, "jobs", j.ID, "job.log"))
		}
		if os.IsNotExist(err) && j.OutputPath != "" {
			f, err = os.Open(filepath.Join(j.OutputPath, "job.log"))
		}
		if err != nil {
			fyne.Do(func() { logText.SetText("No processing log yet.\n" + err.Error()) })
			return nil
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil {
			return err
		}
		if st.Size() > 65536 {
			_, _ = f.Seek(-65536, io.SeekEnd)
		}
		b, err := io.ReadAll(io.LimitReader(f, 65536))
		if err != nil {
			return err
		}
		fyne.Do(func() { logText.SetText(string(b)) })
		return nil
	})
}
