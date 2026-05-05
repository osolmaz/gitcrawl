package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ghCommandCacheStats struct {
	CacheDir string                         `json:"cache_dir"`
	Entries  int                            `json:"entries"`
	Expired  int                            `json:"expired"`
	Locks    int                            `json:"locks"`
	Bytes    int64                          `json:"bytes"`
	Commands map[string]ghCommandCacheCount `json:"commands"`
}

type ghCommandCacheCount struct {
	Entries int   `json:"entries"`
	Bytes   int64 `json:"bytes"`
}

type ghCommandCacheKeyInfo struct {
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"created_at"`
	Age       string    `json:"age"`
	Command   string    `json:"command"`
	Args      []string  `json:"args"`
	Bytes     int64     `json:"bytes"`
	Expired   bool      `json:"expired"`
}

func (a *App) runGHXCache(args []string) error {
	if len(args) == 0 {
		return usageErr(fmt.Errorf("usage: gh xcache <stats|keys|flush>"))
	}
	fs := flag.NewFlagSet("xcache "+args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", false, "write JSON output")
	if err := fs.Parse(args[1:]); err != nil {
		return usageErr(err)
	}
	a.applyCommandJSON(*jsonOut)
	switch args[0] {
	case "stats":
		return a.runGHXCacheStats()
	case "keys":
		return a.runGHXCacheKeys()
	case "flush":
		return a.runGHXCacheFlush()
	default:
		return usageErr(fmt.Errorf("unknown xcache command %q", args[0]))
	}
}

func (a *App) runGHXCacheStats() error {
	stats, err := a.ghCommandCacheStats()
	if err != nil {
		return err
	}
	if a.format == FormatJSON {
		return a.writeJSONValue(stats, "")
	}
	_, err = fmt.Fprintf(a.Stdout, "Cache Dir:       %s\nEntries:         %d\nExpired:         %d\nLocks:           %d\nBytes:           %d\n",
		stats.CacheDir, stats.Entries, stats.Expired, stats.Locks, stats.Bytes)
	if err != nil {
		return err
	}
	if len(stats.Commands) > 0 {
		_, _ = fmt.Fprintln(a.Stdout, "\nCommands:")
		for command, count := range stats.Commands {
			_, _ = fmt.Fprintf(a.Stdout, "  %-16s %d entries / %d bytes\n", command, count.Entries, count.Bytes)
		}
	}
	return nil
}

func (a *App) runGHXCacheKeys() error {
	keys, err := a.ghCommandCacheKeys()
	if err != nil {
		return err
	}
	if a.format == FormatJSON {
		return a.writeJSONValue(keys, "")
	}
	for _, key := range keys {
		if _, err := fmt.Fprintf(a.Stdout, "%s\t%s\t%s\t%s\n", key.Key, key.Age, key.Command, strings.Join(key.Args, " ")); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) runGHXCacheFlush() error {
	removed, err := a.clearGHCommandCacheCount()
	if err != nil {
		return err
	}
	if a.format == FormatJSON {
		return a.writeJSONValue(map[string]any{"removed": removed}, "")
	}
	_, err = fmt.Fprintf(a.Stdout, "Flushed %d cache entrie(s)\n", removed)
	return err
}

func (a *App) ghCommandCacheStats() (ghCommandCacheStats, error) {
	dir, err := a.ghCommandCacheDir()
	if err != nil {
		return ghCommandCacheStats{}, err
	}
	keys, locks, err := a.collectGHCommandCacheKeys(dir)
	if err != nil {
		return ghCommandCacheStats{}, err
	}
	stats := ghCommandCacheStats{CacheDir: dir, Locks: locks, Commands: map[string]ghCommandCacheCount{}}
	for _, key := range keys {
		if key.Expired {
			stats.Expired++
		} else {
			stats.Entries++
		}
		stats.Bytes += key.Bytes
		count := stats.Commands[key.Command]
		count.Entries++
		count.Bytes += key.Bytes
		stats.Commands[key.Command] = count
	}
	return stats, nil
}

func (a *App) ghCommandCacheKeys() ([]ghCommandCacheKeyInfo, error) {
	dir, err := a.ghCommandCacheDir()
	if err != nil {
		return nil, err
	}
	keys, _, err := a.collectGHCommandCacheKeys(dir)
	return keys, err
}

func (a *App) collectGHCommandCacheKeys(dir string) ([]ghCommandCacheKeyInfo, int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0, err
	}
	keys := make([]ghCommandCacheKeyInfo, 0)
	locks := 0
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".lock") {
			locks++
			continue
		}
		if !entry.Type().IsRegular() || !strings.HasSuffix(name, ".json") {
			continue
		}
		key, ok := ghCommandCacheKeyInfoFromDirEntry(dir, entry)
		if ok {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].CreatedAt.After(keys[j].CreatedAt)
	})
	return keys, locks, nil
}

func ghCommandCacheKeyInfoFromDirEntry(dir string, entry os.DirEntry) (ghCommandCacheKeyInfo, bool) {
	name := entry.Name()
	info, err := entry.Info()
	if err != nil {
		return ghCommandCacheKeyInfo{}, false
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return ghCommandCacheKeyInfo{}, false
	}
	var cached ghCommandCacheEntry
	if err := json.Unmarshal(data, &cached); err != nil {
		return ghCommandCacheKeyInfo{}, false
	}
	ttl := ghCommandCacheTTL(cached.Args)
	age := time.Since(cached.CreatedAt)
	return ghCommandCacheKeyInfo{
		Key:       strings.TrimSuffix(name, ".json"),
		CreatedAt: cached.CreatedAt,
		Age:       age.Round(time.Second).String(),
		Command:   ghCommandName(cached.Args),
		Args:      cached.Args,
		Bytes:     info.Size(),
		Expired:   cached.CreatedAt.IsZero() || age > ttl,
	}, true
}
