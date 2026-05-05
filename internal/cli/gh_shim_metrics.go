package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type ghXCacheCounters struct {
	LocalHits              int64            `json:"local_hits"`
	FallbackHits           int64            `json:"fallback_hits"`
	StaleHits              int64            `json:"stale_hits"`
	BackendMisses          int64            `json:"backend_misses"`
	PassThroughWrites      int64            `json:"pass_through_writes"`
	BackendMissesByCommand map[string]int64 `json:"backend_misses_by_command,omitempty"`
	BackendMissesByRoute   map[string]int64 `json:"backend_misses_by_route,omitempty"`
}

func (a *App) ghXCacheCounters() (ghXCacheCounters, error) {
	dir, err := a.ghCommandCacheDir()
	if err != nil {
		return ghXCacheCounters{}, err
	}
	return readGHXCacheCounters(filepath.Join(dir, ghXCacheStatsFile)), nil
}

func (a *App) resetGHXCacheCounters() error {
	dir, err := a.ghCommandCacheDir()
	if err != nil {
		return err
	}
	return writeAtomicFile(filepath.Join(dir, ghXCacheStatsFile), []byte("{}"), 0o600)
}

func (a *App) incrementGHXCacheCounter(name string) error {
	return a.incrementGHXCacheCounterWithArgs(name, nil)
}

func (a *App) incrementGHXCacheBackendMiss(args []string) error {
	return a.incrementGHXCacheCounterWithArgs("backend_misses", args)
}

func (a *App) incrementGHXCacheCounterWithArgs(name string, args []string) error {
	dir, err := a.ghCommandCacheDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, ghXCacheStatsFile)
	lockPath := path + ".lock"
	lock, locked := tryGHCommandCacheLock(lockPath)
	if !locked {
		return nil
	}
	defer func() {
		_ = lock.Close()
		_ = os.Remove(lockPath)
	}()
	stats := readGHXCacheCounters(path)
	switch name {
	case "local_hits":
		stats.LocalHits++
	case "fallback_hits":
		stats.FallbackHits++
	case "stale_hits":
		stats.StaleHits++
	case "backend_misses":
		stats.BackendMisses++
		if len(args) > 0 {
			if stats.BackendMissesByCommand == nil {
				stats.BackendMissesByCommand = map[string]int64{}
			}
			command := ghCommandName(args)
			stats.BackendMissesByCommand[command]++
			if route := ghCommandRoute(args); route != "" {
				if stats.BackendMissesByRoute == nil {
					stats.BackendMissesByRoute = map[string]int64{}
				}
				stats.BackendMissesByRoute[route]++
			}
		}
	case "pass_through_writes":
		stats.PassThroughWrites++
	default:
		return nil
	}
	data, err := json.Marshal(stats)
	if err != nil {
		return err
	}
	return writeAtomicFile(path, data, 0o600)
}

func ghCommandRoute(args []string) string {
	if len(args) == 0 {
		return ""
	}
	if args[0] == "api" {
		return normalizeGHAPIRoute(args[1:])
	}
	if len(args) >= 2 {
		return ghCommandName(args)
	}
	return args[0]
}

func readGHXCacheCounters(path string) ghXCacheCounters {
	data, err := os.ReadFile(path)
	if err != nil {
		return ghXCacheCounters{}
	}
	var stats ghXCacheCounters
	if err := json.Unmarshal(data, &stats); err != nil {
		return ghXCacheCounters{}
	}
	return stats
}

func writeAtomicFile(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Chmod(perm); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func staleGHCommandCacheLock(info os.FileInfo) bool {
	return time.Since(info.ModTime()) > 2*time.Minute
}
