package process

import (
	"runtime"
	"strings"
)

// CommandLine formats a copyable diagnostic command. Windows uses PowerShell
// literal quoting, including the call operator for quoted executable paths.
// Commands are still executed directly with exec.Command, never through a shell.
func CommandLine(exe string, args []string) string {
	quote := func(s string) string {
		if s != "" && strings.IndexFunc(s, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_.:/=+%", r))
		}) < 0 {
			return s
		}
		if runtime.GOOS == "windows" {
			return "'" + strings.ReplaceAll(s, "'", "''") + "'"
		}
		return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
	}
	out := []string{quote(exe)}
	if runtime.GOOS == "windows" {
		out = append([]string{"&"}, out...)
	}
	for _, a := range args {
		out = append(out, quote(a))
	}
	return strings.Join(out, " ")
}
