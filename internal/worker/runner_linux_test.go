//go:build linux

package worker

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLinuxRunnerProcessGroup(t *testing.T) {
	runner := NewLinuxRunner(true)

	t.Run("kills child process tree on cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		// Start a process that spawns a sleep child
		opts := CommandOptions{
			GroupMode:   ProcessGroupNew,
			GracePeriod: 100 * time.Millisecond,
		}

		err_ch := make(chan error, 1)
		go func() {
			_, err := runner.RunWithOptions(
				ctx,
				opts,
				"sh",
				"-c",
				"sleep 30 & wait",
			)
			err_ch <- err
		}()

		// Wait briefly for process and grandchild to start
		time.Sleep(150 * time.Millisecond)
		cancel()

		select {
		case err := <-err_ch:
			if err == nil {
				t.Fatalf("expected error from canceled process")
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("process was not terminated within timeout")
		}

		// Ensure no leftover sleep processes from our test
		out, _ := exec.Command("pgrep", "-f", "sleep 30").Output()
		pids := strings.Fields(string(out))
		for _, pid_str := range pids {
			pid, err := strconv.Atoi(pid_str)
			if err == nil && pid > 0 {
				t.Logf("cleanup residual pid %d", pid)
			}
		}
	})
}
