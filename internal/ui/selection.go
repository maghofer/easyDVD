package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

func selectionModifiers(app fyne.App) fyne.KeyModifier {
	if driver, ok := app.Driver().(desktop.Driver); ok {
		return driver.CurrentKeyModifiers()
	}
	return 0
}

// Both lists use Explorer semantics, with an anchor stable across Shift-clicks.
// Ctrl+Shift adds a range; Shift alone replaces it so ranges can also shrink.
func selectItems(ids []string, selected map[string]bool, anchor *string, target string, modifiers fyne.KeyModifier) bool {
	start, end := -1, -1
	for i, id := range ids {
		if id == *anchor {
			start = i
		}
		if id == target {
			end = i
		}
	}
	if end < 0 {
		return false
	}
	ctrl := modifiers&fyne.KeyModifierControl != 0
	shift := modifiers&fyne.KeyModifierShift != 0
	if shift && start >= 0 {
		if !ctrl {
			clear(selected)
		}
		for i := min(start, end); i <= max(start, end); i++ {
			selected[ids[i]] = true
		}
		return true
	}
	*anchor = target
	if ctrl {
		if selected[target] {
			delete(selected, target)
		} else {
			selected[target] = true
		}
	} else {
		clear(selected)
		selected[target] = true
	}
	return true
}
