package ui

import (
	"net/http"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"easydvd/internal/engine"
	"easydvd/internal/media"
	"easydvd/internal/model"
)

func TestFinishedQualityJobsDoNotRequireEncoderForBalancedQueue(t *testing.T) {
	u := &window{snapshot: engine.Snapshot{Jobs: []model.Job{
		{State: model.Complete, Settings: model.Settings{EncodingProfile: model.ProfileQuality}},
		{State: model.Canceled, Settings: model.Settings{EncodingProfile: model.ProfileQuality}},
		{State: model.Ready, Settings: model.Settings{EncodingProfile: model.ProfileBalanced}},
	}}}
	// No engine is needed: historical jobs must not probe an encoder or show
	// a setup dialog when only Balanced work is runnable.
	if !u.ensureQuality() {
		t.Fatal("finished Quality jobs blocked a Balanced queue")
	}
}

type hcencTransport func(*http.Request) (*http.Response, error)

func (f hcencTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLaunchStartsHCencSetupAndClosingCancelsDownload(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows HCEnc setup")
	}
	t.Setenv("EASYDVD_HCENC", "")
	t.Setenv("LOCALAPPDATA", filepath.Join(t.TempDir(), "local"))
	u := fileListFixture(t)
	u.closed = make(chan struct{})
	defer close(u.closed)
	if err := u.engine.SetTools(media.Tools{FFMS2: "included-plugin", AviSynth: "included-avisynth"}); err != nil {
		t.Fatal(err)
	}
	started, stopped := make(chan struct{}), make(chan struct{})
	original := http.DefaultTransport
	http.DefaultTransport = hcencTransport(func(r *http.Request) (*http.Response, error) {
		close(started)
		<-r.Context().Done()
		close(stopped)
		return nil, r.Context().Err()
	})
	defer func() { http.DefaultTransport = original }()
	u.setupQualityOnLaunch()
	if u.qualityDialog == nil {
		t.Fatal("first launch did not show setup")
	}
	defer u.qualityDialog.Hide()
	u.setupQualityOnLaunch() // A second startup callback must not start another setup.
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("setup waited for a download click")
	}
	u.qualityDialog.Hide()
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("closing setup did not cancel the request")
	}
}

func TestLaunchKeepsExistingAndExplicitQualityRuntimes(t *testing.T) {
	u := fileListFixture(t)
	for _, tc := range []struct{ name, override, encoder string }{
		{"existing", "", "existing-encoder"},
		{"explicit", "custom-runtime", ""},
		{"support-missing", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("EASYDVD_HCENC", tc.override)
			tools := media.Tools{HCenc: tc.encoder}
			if tc.name != "support-missing" {
				tools.FFMS2, tools.AviSynth = "included-plugin", "included-avisynth"
			}
			if err := u.engine.SetTools(tools); err != nil {
				t.Fatal(err)
			}
			u.setupQualityOnLaunch()
			if u.qualityDialog != nil {
				t.Fatal("launch replaced a selected runtime or tried to repair missing support libraries")
			}
		})
	}
}
