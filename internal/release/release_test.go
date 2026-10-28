package release

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompare(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"v1.10.0", "1.9.9", 1}, {"0.1.0", "0.1.0-preview.6", 1},
		{"1.0.0-preview.10", "1.0.0-preview.2", 1}, {"1.0.0", "1.0.0+build.7", 0},
		{"1.0.0-alpha", "1.0.0-alpha.1", -1}, {"2.0.0", "3.0.0", -1},
		{"1.0.0-1", "1.0.0-a", -1}, {"1.0.0-999999999999999999999", "1.0.0-1000000000000000000000", -1},
	} {
		c, err := Compare(tc.a, tc.b)
		if err != nil || c != tc.want {
			t.Errorf("%s vs %s: %d, %v", tc.a, tc.b, c, err)
		}
	}
	for _, v := range []string{"latest", "1.0", "1.0.0-01", "01.0.0", "1.0.0/evil"} {
		if _, err := Compare(v, "1.0.0"); err == nil {
			t.Errorf("accepted %q", v)
		}
	}
}

func TestCheck(t *testing.T) {
	for _, tc := range []struct {
		name, body, current        string
		code                       int
		available, noRelease, fail bool
	}{
		{"new", `{"tag_name":"v1.10.0","html_url":"https://evil.invalid"}`, "1.9.0", 200, true, false, false},
		{"equal", `{"tag_name":"v1.0.0"}`, "1.0.0", 200, false, false, false},
		{"ahead", `{"tag_name":"v1.0.0"}`, "1.1.0-preview.1", 200, false, false, false},
		{"preview to stable", `{"tag_name":"v1.0.0"}`, "1.0.0-preview.1", 200, true, false, false},
		{"none", "", "1.0.0", 404, false, true, false},
		{"rate limit", "", "1.0.0", 403, false, false, true},
		{"server failure", "", "1.0.0", 500, false, false, true},
		{"bad json", "oops", "1.0.0", 200, false, false, true},
		{"invalid tag", `{"tag_name":"main"}`, "1.0.0", 200, false, false, true},
		{"draft", `{"tag_name":"v2.0.0","draft":true}`, "1.0.0", 200, false, false, true},
		{"prerelease", `{"tag_name":"v2.0.0","prerelease":true}`, "1.0.0", 200, false, false, true},
		{"oversized", strings.Repeat(" ", 1<<20+1), "1.0.0", 200, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/repos/example/easyDVD/releases/latest" || r.UserAgent() != "easyDVD/"+tc.current {
					t.Error("unexpected request")
				}
				w.WriteHeader(tc.code)
				fmt.Fprint(w, tc.body)
			}))
			defer s.Close()
			r, err := check(context.Background(), s.Client(), s.URL, "example/easyDVD", tc.current)
			if (err != nil) != tc.fail || r.Available != tc.available || r.NoRelease != tc.noRelease {
				t.Fatalf("%+v, %v", r, err)
			}
			if r.Available && r.URL != "https://github.com/example/easyDVD/releases/tag/v1.10.0" && tc.name == "new" {
				t.Fatal("untrusted release URL")
			}
		})
	}
}

func TestCheckCancellationAndConfiguration(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected request") }))
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := check(ctx, s.Client(), s.URL, "example/easyDVD", "1.0.0"); err == nil {
		t.Fatal("ignored cancellation")
	}
	for _, repo := range []string{"", "../repo", "owner/repo/path"} {
		if _, err := check(context.Background(), s.Client(), s.URL, repo, "1.0.0"); err == nil {
			t.Fatal("invalid repository")
		}
	}
}
