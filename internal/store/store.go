// Package store is an append-only directory of one JSON file per handoff entry.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/FabioLeitao/zero-handoff/internal/redact"
)

// ErrNone means the directory has no entry that matches the filter.
var ErrNone = errors.New("no handoff entry")

// Origin identifies the harness that wrote the entry. show prints these fields
// unchanged so a later harness cannot present the entry as its own.
type Origin struct {
	Harness   string `json:"harness"`
	Version   string `json:"version"`
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
	At        string `json:"at"`
	Resume    string `json:"resume"`
}

// Entry is one append-only handoff record.
type Entry struct {
	Origin Origin `json:"origin"`
	Note   string `json:"note"`
}

// MissingError lists origin fields that were empty. Save writes nothing.
type MissingError struct {
	Fields []string
}

func (e MissingError) Error() string {
	return "missing origin fields: " + strings.Join(e.Fields, ", ")
}

// Validate checks the required origin and a non-empty note.
func (e Entry) Validate() error {
	var missing []string
	if strings.TrimSpace(e.Origin.Harness) == "" {
		missing = append(missing, "harness")
	}
	if strings.TrimSpace(e.Origin.Version) == "" {
		missing = append(missing, "version")
	}
	if strings.TrimSpace(e.Origin.SessionID) == "" {
		missing = append(missing, "sessionId")
	}
	if strings.TrimSpace(e.Origin.Cwd) == "" {
		missing = append(missing, "cwd")
	}
	if strings.TrimSpace(e.Origin.At) == "" {
		missing = append(missing, "at")
	}
	if strings.TrimSpace(e.Origin.Resume) == "" {
		missing = append(missing, "resume")
	}
	if len(missing) > 0 {
		return MissingError{Fields: missing}
	}
	if !strings.Contains(e.Origin.Resume, e.Origin.SessionID) {
		return fmt.Errorf("resume command must contain sessionId")
	}
	if _, err := time.Parse(time.RFC3339Nano, e.Origin.At); err != nil {
		return fmt.Errorf("origin.at must be RFC3339 UTC: %w", err)
	}
	if strings.TrimSpace(e.Note) == "" {
		return errors.New("note is empty")
	}
	return nil
}

// Store is the handoff directory.
type Store struct {
	Dir string
}

// DefaultDir is ${HANDOFF_DIR} or ${XDG_DATA_HOME:-~/.local/share}/handoff.
func DefaultDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("HANDOFF_DIR")); dir != "" {
		return dir, nil
	}
	base := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "handoff"), nil
}

// Save redacts the note and creates a new file. It never rewrites an existing entry.
func (s Store) Save(entry Entry) (string, error) {
	entry.Note = redact.Note(entry.Note)
	if err := entry.Validate(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return "", err
	}
	name := fileName(entry)
	final := filepath.Join(s.Dir, name)
	for i := 0; i < 1000; i++ {
		candidate := final
		if i > 0 {
			candidate = strings.TrimSuffix(final, ".json") + fmt.Sprintf("-%d.json", i)
		}
		tmp := candidate + ".tmp"
		payload, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			return "", err
		}
		payload = append(payload, '\n')
		f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", err
		}
		_, werr := f.Write(payload)
		cerr := f.Close()
		if werr != nil {
			os.Remove(tmp)
			return "", werr
		}
		if cerr != nil {
			os.Remove(tmp)
			return "", cerr
		}
		// Link fails if candidate already exists, so a raced writer cannot replace it.
		if err := os.Link(tmp, candidate); err != nil {
			os.Remove(tmp)
			if os.IsExist(err) {
				continue
			}
			return "", err
		}
		os.Remove(tmp)
		return candidate, nil
	}
	return "", fmt.Errorf("could not allocate a new entry name in %s", s.Dir)
}

// Filter selects entries. Empty fields match every entry.
type Filter struct {
	Harness string
	Cwd     string
}

// Latest returns the newest matching entry. Unreadable or corrupt files are skipped
// and reported to warn so one bad file does not hide a valid entry.
func (s Store) Latest(filter Filter, warn io.Writer) (Entry, error) {
	if warn == nil {
		warn = io.Discard
	}
	names, err := filepath.Glob(filepath.Join(s.Dir, "*.json"))
	if err != nil {
		return Entry{}, err
	}
	sort.Strings(names)
	var found *Entry
	var foundAt time.Time
	var foundName string
	for _, name := range names {
		raw, err := os.ReadFile(name)
		if err != nil {
			fmt.Fprintf(warn, "handoff: skip %s: %v\n", name, err)
			continue
		}
		var entry Entry
		if err := json.Unmarshal(raw, &entry); err != nil {
			fmt.Fprintf(warn, "handoff: skip %s: %v\n", name, err)
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, entry.Origin.At)
		if err != nil {
			fmt.Fprintf(warn, "handoff: skip %s: origin.at: %v\n", name, err)
			continue
		}
		if filter.Harness != "" && entry.Origin.Harness != filter.Harness {
			continue
		}
		if filter.Cwd != "" && entry.Origin.Cwd != filter.Cwd {
			continue
		}
		if found == nil || at.After(foundAt) || (at.Equal(foundAt) && name > foundName) {
			copy := entry
			found = &copy
			foundAt = at
			foundName = name
		}
	}
	if found == nil {
		return Entry{}, ErrNone
	}
	return *found, nil
}

// Format prints origin fields and the note. The harness label stays the writer's.
func Format(entry Entry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "origin.harness: %s\n", entry.Origin.Harness)
	fmt.Fprintf(&b, "origin.version: %s\n", entry.Origin.Version)
	fmt.Fprintf(&b, "origin.sessionId: %s\n", entry.Origin.SessionID)
	fmt.Fprintf(&b, "origin.cwd: %s\n", entry.Origin.Cwd)
	fmt.Fprintf(&b, "origin.at: %s\n", entry.Origin.At)
	fmt.Fprintf(&b, "origin.resume: %s\n", entry.Origin.Resume)
	if strings.Contains(entry.Note, "\n") {
		fmt.Fprintf(&b, "note:\n%s\n", entry.Note)
	} else {
		fmt.Fprintf(&b, "note: %s\n", entry.Note)
	}
	return b.String()
}

func fileName(entry Entry) string {
	stamp := strings.NewReplacer(":", "", "-", "").Replace(entry.Origin.At)
	return fmt.Sprintf("%s-%s-%s.json", stamp, safe(entry.Origin.Harness), safe(entry.Origin.SessionID))
}

func safe(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if len(out) > 48 {
		out = out[:48]
	}
	if out == "" {
		return "x"
	}
	return out
}
