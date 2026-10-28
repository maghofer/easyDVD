package easydvd

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var versionFile string

//go:embed RELEASE_REPOSITORY
var repositoryFile string

// BuildVersion is set by the packaging script; direct Go builds use VERSION.
var BuildVersion string

func Version() string {
	if BuildVersion != "" {
		return BuildVersion
	}
	return strings.TrimSpace(versionFile)
}

func Repository() string { return strings.TrimSpace(repositoryFile) }
