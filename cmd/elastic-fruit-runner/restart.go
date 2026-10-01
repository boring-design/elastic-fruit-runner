package main

import (
	"fmt"
	"log/slog"
	"os"
	"syscall"
)

// restartSelf replaces the current process with a fresh copy of the same binary.
// The PID stays the same, so service managers see one process that keeps running.
// When exec fails the error is returned so the daemon exits non zero and the
// service manager restarts it.
func restartSelf() error {
	exe, err := os.Executable()
	if err != nil {
		slog.Error("exec failed, exiting so the service manager restarts the daemon", "err", err)
		return fmt.Errorf("find daemon executable for restart: %w", err)
	}
	slog.Info("restarting daemon", "path", exe, "args", os.Args)
	if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
		slog.Error("exec failed, exiting so the service manager restarts the daemon", "path", exe, "err", err)
		return fmt.Errorf("restart daemon by exec %s: %w", exe, err)
	}
	return nil
}
