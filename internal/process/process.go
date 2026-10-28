// Package process runs media tools with bounded diagnostics and tree cancellation.
package process

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type tail struct {
	mu sync.Mutex
	b  []byte
}

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.b = append(t.b, p...)
	if len(t.b) > 65536 {
		t.b = append([]byte(nil), t.b[len(t.b)-65536:]...)
	}
	return len(p), nil
}
func (t *tail) String() string { t.mu.Lock(); defer t.mu.Unlock(); return string(t.b) }

type Options struct {
	Dir    string
	Env    []string
	Stdin  io.Reader
	Stdout io.Writer
	Log    io.Writer
	Line   func(string)
}

func Run(ctx context.Context, exe string, args []string, opts Options) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = opts.Dir
	cmd.Stdin = opts.Stdin
	cmd.Env = append(os.Environ(), opts.Env...)
	cmd.WaitDelay = 3 * time.Second
	diagnostics := &tail{}
	cmd.Stderr = diagnostics
	if opts.Log != nil {
		fmt.Fprintf(opts.Log, "\n%s %s\n", exe, strings.Join(args, " "))
		cmd.Stderr = io.MultiWriter(diagnostics, opts.Log)
	}
	cmd.Stdout = opts.Stdout
	var readDone chan struct{}
	if opts.Line != nil {
		pipe, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}
		readDone = make(chan struct{})
		go func() {
			defer close(readDone)
			scan := bufio.NewScanner(pipe)
			scan.Buffer(make([]byte, 4096), 1024*1024)
			for scan.Scan() {
				opts.Line(scan.Text())
			}
		}()
	}
	cleanup, err := prepare(cmd)
	if err != nil {
		return err
	}
	defer cleanup()
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", exe, err)
	}
	if err = attach(cmd); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}
	if ctx.Err() != nil {
		_ = cmd.Cancel()
	}
	if readDone != nil {
		<-readDone
	}
	err = cmd.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("%s: %w\n%s", exe, err, failureDiagnostics(diagnostics.String()))
	}
	return nil
}

// Put an explicit fatal diagnostic first so a short GUI summary doesn't show
// only earlier warnings. Keep the captured context available in job details.
func failureDiagnostics(stderr string) string {
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "err:") || strings.HasPrefix(lower, "error:") || strings.HasPrefix(lower, "fatal:") {
			return line + "\n\n" + stderr
		}
	}
	return stderr
}
