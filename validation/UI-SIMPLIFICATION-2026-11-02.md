# Simplified job controls and gold accents

- Removed the main-window Encoding presets entry point and the job-header
  encoding dropdown. Retained the preset model, SQLite storage, engine methods
  and disconnected dialog implementation for future custom presets.
- Balanced and Quality now use the same inline buttons as other job parameters.
  Batch changes affect only the selected jobs' profile, preserve mixed settings,
  and respect locked jobs. Existing encoding presets remain stored without
  being loaded or applied by the header. Menu presets remain connected.
- Removed Remove selected from the queue. Delete still removes selected jobs;
  Clear queue occupies the former bottom button position and keeps confirmation.
- Replaced blue primary, focus, selection and link accents with warm gold
  (#D4A020), using dark text/icons on filled gold buttons.

Validation: `go test ./...` and `go vet ./...` passed. Updated UI checks cover
mixed selections, both profile buttons, profile-only edits with an existing
encoding preset, retained preset data, menu preset/playback behavior, locked
jobs, Delete removal, disconnected controls and Clear queue placement.
Light/dark and multi-job Fyne render captures were inspected at the normal
1160×780 window size; the controls fit on one parameter row. Captures and build
intermediates are in local scratch storage; test queues were isolated from the GUI.
The Windows app was rebuilt with `scripts/build.ps1 -AppOnly`.

No media pipeline changes or new real-media/player validation were needed for
this UI-only change.
