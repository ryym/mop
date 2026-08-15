package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ryym/mop/internal/api"
	"github.com/ryym/mop/internal/client"
	"github.com/ryym/mop/internal/daemon"
	"github.com/ryym/mop/internal/state"
)

func runDaemon(args []string) error {
	if len(args) == 0 {
		return errors.New("daemon requires a subcommand: start or stop")
	}
	switch args[0] {
	case "start":
		return runDaemonStart(args[1:])
	case "stop":
		return runDaemonStop(args[1:])
	default:
		return fmt.Errorf("unknown daemon subcommand: %s", args[0])
	}
}

func runDaemonStart(args []string) error {
	fs := newFlagSet("daemon start")
	port := fs.Int("port", api.DefaultPort, "port to listen on")
	foreground := fs.Bool("foreground", false, "stay in the foreground and log to the terminal")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if s, err := state.Load(); err == nil {
		if _, err := client.New(s.Port).Status(); err == nil {
			return fmt.Errorf("daemon is already running on port %d (pid %d)", s.Port, s.PID)
		}
		// The state file is stale; the port bind below is the real check.
		_ = state.Clear()
	}

	if !*foreground {
		return startInBackground(*port)
	}

	// Signals are handled here rather than in the daemon package so that the
	// daemon stays a plain library from the caller's point of view.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv, err := daemon.New(daemon.Options{
		Port:   *port,
		Logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	})
	if err != nil {
		return err
	}
	return srv.Run(ctx)
}

// startInBackground spawns the detached daemon and waits until it answers, so
// that a failure to bind the port is reported to the user right here.
func startInBackground(port int) error {
	if err := state.Spawn(port); err != nil {
		return err
	}
	c := client.New(port)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := c.Status(); err == nil {
			fmt.Printf("mop daemon is running on port %d\n", port)
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	logPath, _ := state.LogPath()
	return fmt.Errorf("daemon did not start (see %s)", logPath)
}

func runDaemonStop(args []string) error {
	fs := newFlagSet("daemon stop")
	if err := fs.Parse(args); err != nil {
		return err
	}
	s, err := state.Load()
	if errors.Is(err, state.ErrNoState) {
		return nil // Nothing to stop.
	}
	if err != nil {
		return err
	}
	if err := client.New(s.Port).Shutdown(); err != nil {
		// The daemon may already be gone; drop the stale state file.
		_ = state.Clear()
		return nil
	}
	return nil
}
