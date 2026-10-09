// Package cli is the handoff command. It does not open a socket or dial the network.
package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/FabioLeitao/zero-handoff/internal/store"
)

// Main runs one invocation. args are the arguments after the program name.
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: handoff save|show")
		return 2
	}
	switch args[0] {
	case "save":
		return saveCmd(args[1:], stdin, stderr)
	case "show":
		return showCmd(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "handoff: unknown command %q\n", args[0])
		return 2
	}
}

func saveCmd(args []string, stdin io.Reader, stderr io.Writer) int {
	fs := flag.NewFlagSet("save", flag.ContinueOnError)
	fs.SetOutput(stderr)
	note := fs.String("note", "", "note text; otherwise stdin is the note or a session JSON object")
	harness := fs.String("harness", "", "origin harness")
	version := fs.String("version", "", "origin harness version")
	session := fs.String("session", "", "origin session id")
	cwd := fs.String("cwd", "", "origin working directory")
	resume := fs.String("resume", "", "command that resumes this session in the writing harness")
	skipWithoutSession := fs.Bool("skip-without-session", false, "exit 0 without writing when sessionId is empty")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	raw, err := readStdin(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "handoff: %v\n", err)
		return 2
	}
	entry, err := buildEntry(*note, *harness, *version, *session, *cwd, *resume, string(raw), *skipWithoutSession)
	if errors.Is(err, errSkipNoSession) {
		fmt.Fprintln(stderr, "handoff: no sessionId; nothing saved")
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "handoff: %v\n", err)
		return 2
	}
	dir, err := store.DefaultDir()
	if err != nil {
		fmt.Fprintf(stderr, "handoff: %v\n", err)
		return 2
	}
	if _, err := (store.Store{Dir: dir}).Save(entry); err != nil {
		fmt.Fprintf(stderr, "handoff: %v\n", err)
		return 2
	}
	return 0
}

func showCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	harness := fs.String("harness", "", "keep entries written by this harness")
	cwd := fs.String("cwd", "", "keep entries written from this directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dir, err := store.DefaultDir()
	if err != nil {
		fmt.Fprintf(stderr, "handoff: %v\n", err)
		return 2
	}
	entry, err := (store.Store{Dir: dir}).Latest(store.Filter{Harness: *harness, Cwd: *cwd}, stderr)
	if errors.Is(err, store.ErrNone) {
		fmt.Fprintln(stdout, "no handoff entry")
		return 1
	}
	if err != nil {
		fmt.Fprintf(stderr, "handoff: %v\n", err)
		return 2
	}
	fmt.Fprint(stdout, store.Format(entry))
	return 0
}

func readStdin(stdin io.Reader) ([]byte, error) {
	if stdin == nil {
		return nil, nil
	}
	if f, ok := stdin.(*os.File); ok {
		info, err := f.Stat()
		if err == nil && info.Mode()&os.ModeCharDevice != 0 {
			return nil, nil
		}
	}
	return io.ReadAll(stdin)
}

type sessionPayload struct {
	Event      string `json:"event"`
	SessionID  string `json:"sessionId"`
	Cwd        string `json:"cwd"`
	Note       string `json:"note"`
	StopReason string `json:"stopReason"`
	Error      string `json:"error"`
	Turns      *int   `json:"turns"`
	Harness    string `json:"harness"`
	Version    string `json:"version"`
	Resume     string `json:"resume"`
}

// errSkipNoSession is the --skip-without-session path. Nothing is written
// and no origin field is invented.
var errSkipNoSession = errors.New("no sessionId")

func buildEntry(note, harness, version, session, cwd, resume, raw string, skipWithoutSession bool) (store.Entry, error) {
	var payload *sessionPayload
	text := strings.TrimSpace(raw)
	if text != "" && strings.HasPrefix(text, "{") {
		var parsed sessionPayload
		if err := json.Unmarshal([]byte(text), &parsed); err != nil {
			return store.Entry{}, fmt.Errorf("stdin JSON: %w", err)
		}
		payload = &parsed
	} else if note == "" {
		note = raw
	}
	if payload != nil {
		if session == "" {
			session = payload.SessionID
		}
		if cwd == "" {
			cwd = payload.Cwd
		}
		if harness == "" {
			harness = payload.Harness
		}
		if version == "" {
			version = payload.Version
		}
		if resume == "" {
			resume = payload.Resume
		}
		if note == "" {
			note = payload.Note
		}
		if note == "" {
			note = synthesizedNote(*payload)
		}
	}
	if strings.TrimSpace(session) == "" && skipWithoutSession {
		return store.Entry{}, errSkipNoSession
	}
	if version == "" {
		version = strings.TrimSpace(os.Getenv("HANDOFF_VERSION"))
	}
	if version == "" && harness == "zero" {
		resolved, err := zeroVersion()
		if err != nil {
			return store.Entry{}, err
		}
		version = resolved
	}
	if resume == "" && harness == "zero" && session != "" {
		resume = "zero exec --resume " + session
	}
	entry := store.Entry{
		Origin: store.Origin{
			Harness:   strings.TrimSpace(harness),
			Version:   strings.TrimSpace(version),
			SessionID: strings.TrimSpace(session),
			Cwd:       strings.TrimSpace(cwd),
			At:        time.Now().UTC().Format(time.RFC3339Nano),
			Resume:    strings.TrimSpace(resume),
		},
		Note: strings.TrimSpace(note),
	}
	return entry, nil
}

func synthesizedNote(payload sessionPayload) string {
	parts := []string{"session ended"}
	if payload.StopReason != "" {
		parts = append(parts, "stopReason="+payload.StopReason)
	}
	if payload.Turns != nil {
		parts = append(parts, fmt.Sprintf("turns=%d", *payload.Turns))
	}
	if payload.Error != "" {
		parts = append(parts, "error="+payload.Error)
	}
	return strings.Join(parts, " ")
}

func zeroVersion() (string, error) {
	out, err := exec.Command("zero", "--version").Output()
	if err != nil {
		return "", fmt.Errorf("origin version: pass --version or set HANDOFF_VERSION (%v)", err)
	}
	line := strings.TrimSpace(string(out))
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	if line == "" {
		return "", errors.New("origin version: zero --version printed nothing")
	}
	return line, nil
}
