// Package history records each TTS generation (text, provider, voice, output
// path) as JSON Lines and loads them back for the `attn history` browser.
package history

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// Entry is one recorded TTS generation.
type Entry struct {
	Time       time.Time `json:"ts"`
	Text       string    `json:"text"`        // original user input
	SpokenText string    `json:"spoken_text"` // after --polish / --style transforms
	Provider   string    `json:"provider"`
	Voice      string    `json:"voice"`
	Model      string    `json:"model,omitempty"`
	Style      string    `json:"style,omitempty"`
	Alert      bool      `json:"alert,omitempty"`
	Path       string    `json:"path"` // cached audio artifact
	Bytes      int       `json:"bytes,omitempty"`
	// Legacy is true for entries reconstructed from ~/.tts-output files that
	// predate history recording (no metadata was saved at the time).
	Legacy bool `json:"-"`
	// Missing is true when the referenced audio file no longer exists.
	Missing bool `json:"-"`
}

// Label returns the display text for an entry: the original input, or a
// fallback for legacy entries.
func (e Entry) Label() string {
	if e.Text != "" {
		return e.Text
	}
	if e.Legacy {
		return filepath.Base(e.Path)
	}
	return "(no text)"
}

// DefaultOutputDir is where attn caches generated audio by default.
func DefaultOutputDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".tts-output")
}

// Path returns the JSONL history file location, honouring XDG_DATA_HOME.
func Path() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "attn", "history.jsonl")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "attn", "history.jsonl")
}

// withFileLock serializes access to the history file across concurrent attn
// processes. Locking is best-effort: if the lock cannot be taken (exotic
// filesystem), fn still runs so TTS is never blocked.
func withFileLock(fn func() error) error {
	path := Path()
	if path == "" {
		return errors.New("cannot resolve history path")
	}
	lf, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fn()
	}
	defer lf.Close()
	if err := unix.Flock(int(lf.Fd()), unix.LOCK_EX); err != nil {
		return fn()
	}
	defer func() { _ = unix.Flock(int(lf.Fd()), unix.LOCK_UN) }()
	return fn()
}

// Record appends one entry to the JSONL history file. It never fails hard:
// recording must not break TTS playback, so errors are returned for the
// caller to log at most.
func Record(e Entry) error {
	return withFileLock(func() error {
		return recordLocked(e)
	})
}

func recordLocked(e Entry) error {
	path := Path()
	if path == "" {
		return errors.New("cannot resolve history path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create history dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer f.Close()
	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write history: %w", err)
	}
	return nil
}

// Load reads the JSONL history file. Corrupt lines are skipped so a partial
// write never blanks the whole history. Entries are returned newest-first,
// merged with legacy entries discovered in the default output dir.
func Load() ([]Entry, error) {
	entries, known := loadJSONL(Path())
	for _, e := range scanLegacy(DefaultOutputDir(), known) {
		entries = append(entries, e)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Time.After(entries[j].Time)
	})
	for i := range entries {
		if _, err := os.Stat(entries[i].Path); err != nil {
			entries[i].Missing = true
		}
	}
	return entries, nil
}

// loadJSONL parses the history file, returning entries (file order) and the
// set of referenced audio paths.
func loadJSONL(path string) ([]Entry, map[string]bool) {
	known := make(map[string]bool)
	if path == "" {
		return nil, known
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, known
	}
	defer f.Close()

	var entries []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue // skip corrupt line
		}
		if e.Path == "" {
			continue
		}
		entries = append(entries, e)
		known[e.Path] = true
	}
	return entries, known
}

// scanLegacy finds audio files in dir that have no history record and
// synthesizes legacy entries for them. Timestamps come from the default
// <unixnano>.<ext> naming scheme, falling back to file mtime.
func scanLegacy(dir string, known map[string]bool) []Entry {
	if dir == "" {
		return nil
	}
	var out []Entry
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".mp3" && ext != ".wav" {
			return nil
		}
		if known[path] {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		ts := info.ModTime()
		base := strings.TrimSuffix(filepath.Base(path), ext)
		if ns, err := strconv.ParseInt(base, 10, 64); err == nil && ns > 1_000_000_000_000_000_000 {
			ts = time.Unix(0, ns)
		}
		out = append(out, Entry{
			Time:   ts,
			Path:   path,
			Legacy: true,
			Bytes:  int(info.Size()),
		})
		return nil
	})
	return out
}

// Delete removes the entry's audio file and its history record line.
// The entry is identified by its path (unique per generation).
func Delete(e Entry) error {
	return withFileLock(func() error {
		return deleteLocked(e)
	})
}

func deleteLocked(e Entry) error {
	if e.Path == "" {
		return errors.New("entry has no path")
	}
	if err := os.Remove(e.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove audio file: %w", err)
	}
	if e.Legacy {
		return nil // no JSONL line to remove
	}
	path := Path()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read history: %w", err)
	}
	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var other Entry
		if err := json.Unmarshal([]byte(trimmed), &other); err == nil && other.Path == e.Path {
			continue
		}
		kept = append(kept, trimmed)
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("write history: %w", err)
	}
	if _, err := f.WriteString(strings.Join(kept, "\n") + "\n"); err != nil {
		f.Close()
		return fmt.Errorf("write history: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("sync history: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close history: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace history: %w", err)
	}
	return nil
}
