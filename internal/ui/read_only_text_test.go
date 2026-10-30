package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func assertReadableDiagnostic(t *testing.T, u *window, entry *widget.Entry) {
	t.Helper()
	before := entry.Text
	entry.TypedRune('x')
	if !entry.Disabled() || entry.Text != before {
		t.Fatal("diagnostic text is editable")
	}
	actual := entry.Theme().Color(theme.ColorNameDisabled, u.app.Settings().ThemeVariant())
	want := theme.ForegroundColor()
	r, g, b, a := actual.RGBA()
	wr, wg, wb, wa := want.RGBA()
	if r != wr || g != wg || b != wb || a != wa {
		t.Fatal("read-only diagnostics still use dim disabled text")
	}
}

func TestDiagnosticDialogsUseReadableText(t *testing.T) {
	u := fileListFixture(t)
	if err := os.MkdirAll(u.engine.LogsDir(), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(u.engine.JobLogPath(u.selected), []byte("Scanning sources\n18 videos inspected\nReady for encoding\n"), 0644); err != nil {
		t.Fatal(err)
	}
	preview := func(name string) {
		if dir := os.Getenv("EASYDVD_UI_PREVIEW"); dir != "" {
			f, err := os.Create(filepath.Join(dir, name+".png"))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, u.win.Canvas().Capture()); err != nil {
				t.Fatal(err)
			}
			f.Close()
		}
	}
	closeDialog := func() {
		var close *widget.Button
		walkFileTestObjects(u.win.Canvas().Overlays().Top(), func(object fyne.CanvasObject) {
			if b, ok := object.(*widget.Button); ok && b.Text == "Close" {
				close = b
			}
		})
		if close == nil {
			t.Fatal("dialog close button missing")
		}
		test.Tap(close)
	}
	for _, appearance := range []string{"Light", "Dark"} {
		u.applyTheme(appearance)
		u.refresh()
		u.jobDetails()
		var tabs *container.AppTabs
		walkFileTestObjects(u.win.Canvas().Overlays().Top(), func(object fyne.CanvasObject) {
			if found, ok := object.(*container.AppTabs); ok {
				tabs = found
			}
		})
		if tabs == nil {
			t.Fatal("details tabs missing")
		}
		for i := range tabs.Items {
			tabs.SelectIndex(i)
			entries := 0
			walkFileTestObjects(tabs.Items[i].Content, func(object fyne.CanvasObject) {
				if entry, ok := object.(*widget.Entry); ok {
					assertReadableDiagnostic(t, u, entry)
					entries++
				}
			})
			if entries == 0 {
				t.Fatal("details text missing")
			}
			name := "details"
			if i == 1 {
				name = "log"
			}
			preview("diagnostic-" + name + "-" + appearance)
		}
		closeDialog()
		u.diagnostics()
		deadline := time.Now().Add(3 * time.Second)
		for {
			var entry *widget.Entry
			walkFileTestObjects(u.win.Canvas().Overlays().Top(), func(object fyne.CanvasObject) {
				if e, ok := object.(*widget.Entry); ok {
					entry = e
				}
			})
			if entry != nil {
				assertReadableDiagnostic(t, u, entry)
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("tools diagnostics did not load")
			}
			time.Sleep(5 * time.Millisecond)
		}
		preview("diagnostic-tools-" + appearance)
		closeDialog()
	}
}
