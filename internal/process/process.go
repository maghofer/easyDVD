// Package process runs media tools with bounded diagnostics and tree cancellation.
package process

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
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

// Metrics reports process CPU time independently of wall time. CPUPercent is
// normalized to all logical CPUs, matching system-wide CPU utilization displays.
type Metrics struct {
	Executable string   `json:"executable"`
	Args       []string `json:"args"`
	Seconds    float64  `json:"seconds"`
	CPUSeconds float64  `json:"cpu_seconds"`
	CPUPercent float64  `json:"cpu_percent"`
}

type observerKey struct{}

func Observe(ctx context.Context, fn func(Metrics)) context.Context {
	return context.WithValue(ctx, observerKey{}, fn)
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
	started := time.Now()
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
	if observer, ok := ctx.Value(observerKey{}).(func(Metrics)); ok && cmd.ProcessState != nil {
		elapsed := time.Since(started).Seconds()
		cpu := (cmd.ProcessState.UserTime() + cmd.ProcessState.SystemTime()).Seconds()
		observer(Metrics{Executable: exe, Args: append([]string(nil), args...), Seconds: elapsed, CPUSeconds: cpu, CPUPercent: cpu / elapsed / float64(runtime.NumCPU()) * 100})
	}
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
