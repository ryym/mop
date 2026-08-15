// Package state owns the state file and the "is the daemon alive?" logic.
package state

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// ErrNoState means no state file exists, i.e. no daemon has been started.
var ErrNoState = errors.New("no daemon state file")

// State is what the daemon writes on startup so that the CLI can find it.
type State struct {
	Port    int    `json:"port"`
	PID     int    `json:"pid"`
	Version string `json:"version"`
}

// Dir is the directory holding the state file and the log.
// XDG_STATE_HOME is honoured, defaulting to ~/.local/state.
func Dir() (string, error) {
	if base := os.Getenv("XDG_STATE_HOME"); base != "" {
		return filepath.Join(base, "mop"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "mop"), nil
}

func FilePath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemon.json"), nil
}

func LogPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mop.log"), nil
}

// Load reads the state file. It returns ErrNoState when there is none.
func Load() (State, error) {
	var s State
	path, err := FilePath()
	if err != nil {
		return s, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return s, ErrNoState
		}
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		// A corrupted state file is treated as no state at all: the CLI will
		// start a fresh daemon and overwrite it.
		return s, ErrNoState
	}
	return s, nil
}

// Save writes the state file, creating the directory if needed.
func Save(s State) error {
	path, err := FilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Clear removes the state file. A missing file is not an error.
func Clear() error {
	path, err := FilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
