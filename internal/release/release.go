// Package release implements the user-triggered check for stable GitHub releases.
package release

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var semver = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
var repository = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func parse(v string) ([]string, error) {
	m := semver.FindStringSubmatch(v)
	if m == nil {
		return nil, fmt.Errorf("invalid release version %q", v)
	}
	for _, id := range strings.Split(m[4], ".") {
		if numeric(id) && len(id) > 1 && id[0] == '0' {
			return nil, fmt.Errorf("invalid release version %q", v)
		}
	}
	return m, nil
}

func numeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func compareNumber(a, b string) int {
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return strings.Compare(a, b)
}

// Compare follows SemVer precedence, including numeric prerelease identifiers.
func Compare(a, b string) (int, error) {
	x, err := parse(a)
	if err != nil {
		return 0, err
	}
	y, err := parse(b)
	if err != nil {
		return 0, err
	}
	for i := 1; i <= 3; i++ {
		if c := compareNumber(x[i], y[i]); c != 0 {
			return c, nil
		}
	}
	if x[4] == y[4] {
		return 0, nil
	}
	if x[4] == "" {
		return 1, nil
	}
	if y[4] == "" {
		return -1, nil
	}
	xs, ys := strings.Split(x[4], "."), strings.Split(y[4], ".")
	for i := 0; i < len(xs) && i < len(ys); i++ {
		a, b := xs[i], ys[i]
		c := strings.Compare(a, b)
		if numeric(a) && numeric(b) {
			c = compareNumber(a, b)
		} else if numeric(a) {
			c = -1
		} else if numeric(b) {
			c = 1
		}
		if c != 0 {
			return c, nil
		}
	}
	if len(xs) < len(ys) {
		return -1, nil
	}
	return 1, nil
}

type Result struct {
	Version, URL         string
	Available, NoRelease bool
}

// Check makes one bounded request and never downloads or installs an update.
func Check(ctx context.Context, repo, current string) (Result, error) {
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return check(ctx, client, "https://api.github.com", repo, current)
}

func check(ctx context.Context, client *http.Client, api, repo, current string) (Result, error) {
	if !repository.MatchString(repo) {
		return Result{}, fmt.Errorf("the GitHub release repository is not configured correctly")
	}
	if _, err := parse(current); err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api+"/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "easyDVD/"+current)
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("could not contact GitHub: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Result{NoRelease: true}, nil
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return Result{}, fmt.Errorf("GitHub refused the request or its rate limit was reached; try again later")
	}
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("GitHub returned HTTP %d; try again later", resp.StatusCode)
	}
	var r struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if err != nil {
		return Result{}, fmt.Errorf("could not read GitHub response: %w", err)
	}
	if len(data) > 1<<20 {
		return Result{}, fmt.Errorf("GitHub response is too large")
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return Result{}, fmt.Errorf("invalid GitHub release response")
	}
	v, err := parse(r.Tag)
	if err != nil {
		return Result{}, err
	}
	if r.Draft || r.Prerelease || v[4] != "" {
		return Result{}, fmt.Errorf("GitHub did not return a stable release")
	}
	c, err := Compare(r.Tag, current)
	if err != nil {
		return Result{}, err
	}
	// Construct the URL from the configured repository; never open a response-supplied URL.
	return Result{Version: strings.TrimPrefix(r.Tag, "v"), URL: "https://github.com/" + repo + "/releases/tag/" + r.Tag, Available: c > 0}, nil
}
