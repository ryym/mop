package state

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ryym/mop/internal/api"
	"github.com/ryym/mop/internal/client"
	"github.com/ryym/mop/internal/version"
)

// startupTimeout is how long we wait for a freshly spawned daemon to answer.
const startupTimeout = 5 * time.Second

// Connect returns a client for a running daemon, starting one if none is
// running or the one that is turns out to be unusable.
func Connect() (*client.Client, error) {
	s, err := Load()
	if errors.Is(err, ErrNoState) {
		return startAndWait(api.DefaultPort)
	}
	if err != nil {
		return nil, err
	}

	c := client.New(s.Port)
	status, err := c.Status()
	if err != nil {
		_ = Clear()
		return startAndWait(api.DefaultPort)
	}
	// Replace a daemon running from another binary: it would otherwise serve
	// the assets it was built with, or fail in confusing, protocol shaped ways.
	if status.Version != version.Version || s.Build != BuildID() {
		_ = c.Shutdown()
		waitGone(c)
		_ = Clear()
		// Keep whatever port the user had chosen for the old daemon.
		return startAndWait(s.Port)
	}
	return c, nil
}

// Spawn starts the daemon as a detached background process running this same
// binary, logging to the daemon log file. It returns once the process has been
// started, which is before the daemon is reachable.
func Spawn(port int) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	logFile, err := openLog()
	if err != nil {
		return err
	}
	defer logFile.Close()

	// Pass --foreground so the child *is* the daemon rather than forking again:
	// detaching it is this function's job.
	cmd := exec.Command(exe, "daemon", "start", "--foreground", "--port", fmt.Sprint(port))
	cmd.Stdin = nil
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	// Give it its own process group so it outlives the CLI process.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	// The daemon detaches itself; reap the intermediate process state.
	go func() { _ = cmd.Wait() }()
	return nil
}

func openLog() (*os.File, error) {
	path, err := LogPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
}

func startAndWait(port int) (*client.Client, error) {
	if err := Spawn(port); err != nil {
		return nil, fmt.Errorf("failed to start daemon: %w", err)
	}
	c := client.New(port)
	deadline := time.Now().Add(startupTimeout)
	for time.Now().Before(deadline) {
		if _, err := c.Status(); err == nil {
			return c, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	logPath, _ := LogPath()
	return nil, fmt.Errorf("daemon did not come up within %s (see %s)", startupTimeout, logPath)
}

func waitGone(c *client.Client) {
	deadline := time.Now().Add(startupTimeout)
	for time.Now().Before(deadline) {
		if _, err := c.Status(); err != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}
