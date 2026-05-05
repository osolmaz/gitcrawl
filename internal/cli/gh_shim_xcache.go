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
	CacheDir           string                         `json:"cache_dir"`
	Entries            int                            `json:"entries"`
	Expired            int                            `json:"expired"`
	Locks              int                            `json:"locks"`
	Bytes              int64                          `json:"bytes"`
	Since              string                         `json:"since,omitempty"`
	CacheHits          int64                          `json:"cache_hits"`
	TotalReads         int64                          `json:"total_reads"`
	HitRatePercent     float64                        `json:"hit_rate_percent"`
	Counters           ghXCacheCounters               `json:"counters"`
	CumulativeCounters *ghXCacheCounters              `json:"cumulative_counters,omitempty"`
	Commands           map[string]ghCommandCacheCount `json:"commands"`
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
	Tags      []string  `json:"tags,omitempty"`
	Bytes     int64     `json:"bytes"`
	Expired   bool      `json:"expired"`
}

func (a *App) runGHXCache(args []string) error {
	if len(args) == 0 {
		return usageErr(fmt.Errorf("usage: gh xcache <stats|keys|gc|flush|reset|snapshot>"))
	}
	fs := flag.NewFlagSet("xcache "+args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", false, "write JSON output")
	sinceRaw := fs.String("since", "", "show stats for the recent duration (stats only)")
	resetAfterSnapshot := fs.Bool("reset", false, "reset counters after writing a snapshot (snapshot only)")
	if err := fs.Parse(args[1:]); err != nil {
		return usageErr(err)
	}
	a.applyCommandJSON(*jsonOut)
	switch args[0] {
	case "stats":
		var since time.Duration
		if strings.TrimSpace(*sinceRaw) != "" {
			parsed, err := time.ParseDuration(strings.TrimSpace(*sinceRaw))
			if err != nil || parsed <= 0 {
				return usageErr(fmt.Errorf("invalid --since duration %q", *sinceRaw))
			}
			since = parsed
		}
		return a.runGHXCacheStats(since)
	case "keys":
		return a.runGHXCacheKeys()
	case "gc":
		return a.runGHXCacheGC()
	case "flush":
		return a.runGHXCacheFlush()
	case "reset":
		return a.runGHXCacheReset()
	case "snapshot":
		return a.runGHXCacheSnapshot(*resetAfterSnapshot)
	default:
		return usageErr(fmt.Errorf("unknown xcache command %q", args[0]))
	}
}

func (a *App) runGHXCacheStats(since time.Duration) error {
	stats, err := a.ghCommandCacheStats(since)
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
	if stats.Since != "" {
		_, _ = fmt.Fprintf(a.Stdout, "\nSince: %s\n", stats.Since)
	}
	_, _ = fmt.Fprintf(a.Stdout, "\nCounters:\n  local hits:          %d\n  fallback hits:       %d\n  stale hits:          %d\n  backend misses:      %d\n  pass-through writes: %d\n  hit rate:            %.1f%% (%d/%d reads)\n",
		stats.Counters.LocalHits, stats.Counters.FallbackHits, stats.Counters.StaleHits, stats.Counters.BackendMisses, stats.Counters.PassThroughWrites,
		stats.HitRatePercent, stats.CacheHits, stats.TotalReads)
	printGHXCacheMisses(a.Stdout, "Backend Misses by Command", stats.Counters.BackendMissesByCommand)
	printGHXCacheMisses(a.Stdout, "Backend Misses by Route", stats.Counters.BackendMissesByRoute)
	printGHXCacheMisses(a.Stdout, "Backend Misses by Key", stats.Counters.BackendMissesByKey)
	return nil
}

func printGHXCacheMisses(w io.Writer, title string, misses map[string]int64) {
	if len(misses) == 0 {
		return
	}
	type row struct {
		name  string
		count int64
	}
	rows := make([]row, 0, len(misses))
	for name, count := range misses {
		rows = append(rows, row{name: name, count: count})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].count == rows[j].count {
			return rows[i].name < rows[j].name
		}
		return rows[i].count > rows[j].count
	})
	_, _ = fmt.Fprintf(w, "\n%s:\n", title)
	for index, row := range rows {
		if index >= 10 {
			break
		}
		_, _ = fmt.Fprintf(w, "  %-40s %d\n", row.name, row.count)
	}
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

func (a *App) runGHXCacheReset() error {
	if err := a.resetGHXCacheCounters(); err != nil {
		return err
	}
	if a.format == FormatJSON {
		return a.writeJSONValue(map[string]any{"reset": true}, "")
	}
	_, err := fmt.Fprintln(a.Stdout, "Reset xcache counters")
	return err
}

type ghCommandCacheSnapshotResult struct {
	SnapshotPath string `json:"snapshot_path"`
	Reset        bool   `json:"reset"`
}

func (a *App) runGHXCacheSnapshot(reset bool) error {
	stats, err := a.ghCommandCacheStats(0)
	if err != nil {
		return err
	}
	dir, err := a.ghCommandCacheDir()
	if err != nil {
		return err
	}
	snapshotDir := filepath.Join(dir, "_snapshots")
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(snapshotDir, time.Now().UTC().Format("20060102T150405Z")+".json")
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomicFile(path, data, 0o600); err != nil {
		return err
	}
	if reset {
		if err := a.resetGHXCacheCounters(); err != nil {
			return err
		}
	}
	result := ghCommandCacheSnapshotResult{SnapshotPath: path, Reset: reset}
	if a.format == FormatJSON {
		return a.writeJSONValue(result, "")
	}
	_, err = fmt.Fprintf(a.Stdout, "Wrote xcache snapshot: %s\n", path)
	if err == nil && reset {
		_, err = fmt.Fprintln(a.Stdout, "Reset xcache counters")
	}
	return err
}

type ghCommandCacheGCResult struct {
	Removed      int `json:"removed"`
	LocksRemoved int `json:"locks_removed"`
}

func (a *App) runGHXCacheGC() error {
	result, err := a.gcGHCommandCache()
	if err != nil {
		return err
	}
	if a.format == FormatJSON {
		return a.writeJSONValue(result, "")
	}
	_, err = fmt.Fprintf(a.Stdout, "Removed %d expired entrie(s), %d stale lock(s)\n", result.Removed, result.LocksRemoved)
	return err
}

func (a *App) ghCommandCacheStats(since time.Duration) (ghCommandCacheStats, error) {
	dir, err := a.ghCommandCacheDir()
	if err != nil {
		return ghCommandCacheStats{}, err
	}
	keys, locks, err := a.collectGHCommandCacheKeys(dir)
	if err != nil {
		return ghCommandCacheStats{}, err
	}
	counters, _ := a.ghXCacheCounters()
	cumulative := counters
	stats := ghCommandCacheStats{CacheDir: dir, Locks: locks, Counters: counters, Commands: map[string]ghCommandCacheCount{}}
	if since > 0 {
		stats.Since = since.String()
		stats.CumulativeCounters = &cumulative
		stats.Counters = counters.since(since, time.Now())
	}
	stats.CacheHits = stats.Counters.LocalHits + stats.Counters.FallbackHits + stats.Counters.StaleHits
	stats.TotalReads = stats.CacheHits + stats.Counters.BackendMisses
	if stats.TotalReads > 0 {
		stats.HitRatePercent = float64(stats.CacheHits) / float64(stats.TotalReads) * 100
	}
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

func (a *App) gcGHCommandCache() (ghCommandCacheGCResult, error) {
	dir, err := a.ghCommandCacheDir()
	if err != nil {
		return ghCommandCacheGCResult{}, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ghCommandCacheGCResult{}, err
	}
	var result ghCommandCacheGCResult
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(dir, name)
		if strings.HasSuffix(name, ".lock") {
			info, err := entry.Info()
			if err == nil && staleGHCommandCacheLock(info) {
				if err := os.Remove(path); err == nil {
					result.LocksRemoved++
				}
			}
			continue
		}
		if !entry.Type().IsRegular() || !isGHCommandCacheEntryFile(name) {
			continue
		}
		key, ok := ghCommandCacheKeyInfoFromDirEntry(dir, entry)
		if ok && key.Expired {
			if err := os.Remove(path); err == nil {
				result.Removed++
			}
		}
	}
	return result, nil
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
		if !entry.Type().IsRegular() || !isGHCommandCacheEntryFile(name) {
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
		Tags:      cached.Tags,
		Bytes:     info.Size(),
		Expired:   cached.CreatedAt.IsZero() || age > ghCommandCacheEntryTTL(cached, ttl),
	}, true
}
