package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestExplorerSelectionInJobsAndFiles(t *testing.T) {
	for _, jobs := range []bool{true, false} {
		name := "files"
		if jobs {
			name = "jobs"
		}
		t.Run(name, func(t *testing.T) {
			u := jobFixture(t)
			var ids []string
			selection := u.selectedFiles
			click := func(index int, modifiers fyne.KeyModifier) {
				if modifiers == 0 {
					row := newFileRow(u)
					row.update(u.selectedJob(), index)
					test.Tap(row)
				} else {
					u.selectFiles(ids[index], modifiers)
				}
			}
			selectAll := func() { u.files.TypedShortcut(&fyne.ShortcutSelectAll{}) }
			if jobs {
				selection = u.selectedJobs
				for _, j := range u.snapshot.Jobs {
					ids = append(ids, j.ID)
				}
				click = func(index int, modifiers fyne.KeyModifier) {
					if modifiers == 0 {
						row := newJobTile(u)
						row.update(u.snapshot.Jobs[index])
						test.Tap(row)
					} else {
						u.selectJobs(ids[index], modifiers)
					}
				}
				selectAll = func() { u.queue.TypedShortcut(&fyne.ShortcutSelectAll{}) }
			} else {
				for _, v := range u.selectedJob().Videos {
					ids = append(ids, v.ID)
				}
			}
			for _, tc := range []struct {
				index     int
				modifiers fyne.KeyModifier
				want      []int
			}{
				{1, 0, []int{1}},
				{3, 0, []int{3}},
				{3, 0, []int{3}}, // Repeated plain clicks never toggle the item off.
				{1, fyne.KeyModifierControl, []int{1, 3}},
				{3, fyne.KeyModifierControl, []int{1}},
				{0, fyne.KeyModifierShift, []int{0, 1, 2, 3}},
				{2, fyne.KeyModifierShift, []int{2, 3}}, // Shrink the previous range.
				{0, fyne.KeyModifierControl, []int{0, 2, 3}},
				{1, fyne.KeyModifierControl | fyne.KeyModifierShift, []int{0, 1, 2, 3}},
				{4, 0, []int{4}},
			} {
				click(tc.index, tc.modifiers)
				if len(selection) != len(tc.want) {
					t.Fatalf("click %d with modifiers %v: selection=%v wanted=%v", tc.index, tc.modifiers, selection, tc.want)
				}
				for _, index := range tc.want {
					if !selection[ids[index]] {
						t.Fatalf("expected item %d selected", index)
					}
				}
			}
			selectAll()
			if len(selection) != len(ids) {
				t.Fatal("Ctrl+A did not select the complete list")
			}
			click(2, 0)
			if len(selection) != 1 || !selection[ids[2]] {
				t.Fatal("plain click did not replace Select all")
			}
		})
	}
}

func TestJobHoverNeverChangesDisplayedJob(t *testing.T) {
	u := jobFixture(t)
	u.selectJobs(u.snapshot.Jobs[0].ID, 0)
	u.selectFiles("1", 0)
	job, title := u.selected, u.title.Text
	// Fyne highlights rows for both mouse hover and keyboard movement.
	// Highlighting must never be treated as choosing another job.
	u.queue.Highlight(3)
	if u.selected != job || u.title.Text != title || len(u.selectedJobs) != 1 || !u.selectedFiles["1"] {
		t.Fatal("hover/highlight changed the displayed job or its file selection")
	}
	row := newJobTile(u)
	row.update(u.snapshot.Jobs[3])
	test.Tap(row)
	if u.selected != u.snapshot.Jobs[3].ID || u.title.Text != u.snapshot.Jobs[3].Title || len(u.selectedFiles) != 0 {
		t.Fatal("clicking a job did not switch the displayed job")
	}
	walkFileTestObjects(u.win.Content(), func(object fyne.CanvasObject) {
		if _, ok := object.(*widget.Check); ok {
			t.Fatal("list selection still displays a checkbox")
		}
		if b, ok := object.(*widget.Button); ok && (b.Text == "More info" || b.Text == "Less info") {
			t.Fatal("job header still expands")
		}
	})
}
