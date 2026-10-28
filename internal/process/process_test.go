package process

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestProcessHelper(t *testing.T) {
	if os.Getenv("EASYDVD_PROCESS_TEST") != "1" {
		return
	}
	if len(os.Args) > 0 && os.Args[len(os.Args)-1] == "spawn" {
		exe, _ := os.Executable()
		cmd := exec.Command(exe, "-test.run=TestProcessHelper", "--", "child")
		if err := cmd.Start(); err != nil {
			os.Exit(2)
		}
		fmt.Println(cmd.Process.Pid)
	} else {
		fmt.Println("ready")
	}
	time.Sleep(30 * time.Second)
	os.Exit(0)
}
func TestCancelProcessTree(t *testing.T) {
	exe, _ := os.Executable()
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	pid := 0
	start := time.Now()
	err := Run(ctx, exe, []string{"-test.run=TestProcessHelper", "--", "spawn"}, Options{Env: []string{"EASYDVD_PROCESS_TEST=1"}, Line: func(line string) { pid, _ = strconv.Atoi(strings.TrimSpace(line)) }})
	if err != context.DeadlineExceeded {
		t.Fatal(err)
	}
	if time.Since(start) > 4*time.Second {
		t.Fatal("slow cancellation")
	}
	if pid == 0 {
		t.Fatal("helper did not start")
	}
	if processAlive(pid) {
		t.Fatalf("grandchild %d survived cancellation", pid)
	}
}

func TestFatalDiagnosticBeforeEarlierWarnings(t *testing.T) {
	log := strings.Repeat("WARN: Audio timestamps require attention\n", 50) +
		"ERR:  Can only have 128 commands for pre, post, and cell commands.\n"
	got := failureDiagnostics(log)
	if !strings.HasPrefix(got, "ERR:  Can only have 128 commands") || !strings.Contains(got, "WARN: Audio timestamps") {
		t.Fatal("fatal cause must be visible while retaining earlier context", got)
	}
}
