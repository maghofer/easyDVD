package ui

import (
	"context"
	"easydvd/internal/fsops"
	"easydvd/internal/media"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// Classify off the UI thread: stat may need to contact a network share.
func (u *window) addDroppedPaths(paths []string) {
	u.action(func() error {
		var files []string
		for _, path := range paths {
			info, err := fsops.Stat(context.Background(), path)
			if err != nil {
				return err
			}
			if !info.Exists {
				return fmt.Errorf("source does not exist: %s", path)
			}
			if info.Directory {
				if err = u.engine.Add(path); err != nil {
					return err
				}
			} else if media.IsVideo(path) {
				files = append(files, path)
			}
		}
		if len(files) == 1 {
			return u.engine.AddFiles(files)
		}
		if len(files) > 1 {
			fyne.Do(func() { u.chooseFileGrouping(files) })
		}
		return nil
	})
}

func (u *window) chooseFileGrouping(files []string) {
	choice := widget.NewRadioGroup([]string{"All files in one DVD set", "A separate DVD set for each file"}, nil)
	choice.SetSelected(choice.Options[0])
	dialog.NewCustomConfirm("Add video files", "Add files", "Cancel", container.NewVBox(widget.NewLabel(fmt.Sprintf("How should these %d videos be queued?", len(files))), choice), func(ok bool) {
		if !ok {
			return
		}
		together := choice.Selected == choice.Options[0]
		u.action(func() error {
			if together {
				return u.engine.AddFiles(files)
			}
			for _, path := range files {
				if err := u.engine.AddFiles([]string{path}); err != nil {
					return err
				}
			}
			return nil
		})
	}, u.win).Show()
}
