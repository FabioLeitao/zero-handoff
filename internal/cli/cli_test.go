package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestSaveRequiresHarness(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HANDOFF_DIR", dir)
	t.Setenv("HANDOFF_VERSION", "")
	var stdout, stderr bytes.Buffer
	stdin := strings.NewReader(`{"event":"sessionEnd","sessionId":"zero_test_1","cwd":"/tmp/proj","stopReason":"error","turns":1}`)
	code := Main([]string{"save"}, stdin, &stdout, &stderr)
	if code == 0 {
		t.Fatal("save without --harness succeeded")
	}
	if !strings.Contains(stderr.String(), "harness") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	showOut := bytes.Buffer{}
	if Main([]string{"show"}, nil, &showOut, &stderr) != 1 {
		t.Fatalf("show after rejected save: %s", showOut.String())
	}
	if showOut.String() != "no handoff entry\n" {
		t.Fatalf("show = %q", showOut.String())
	}
}

func TestSaveShowRedactsAndKeepsOrigin(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HANDOFF_DIR", dir)
	var stdout, stderr bytes.Buffer
	note := "path /tmp/proj token stays and commit dfd1fe8a1b2c3d4e5f60718293a4b5c6d7e8f901 sk-test-ZH-EVAL-9f3c1a7e-TOKEN"
	code := Main([]string{
		"save",
		"--harness", "zero",
		"--version", "zero 0.9.0",
		"--session", "zero_test_1",
		"--cwd", "/tmp/proj",
		"--note", note,
	}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("save code %d stderr %s", code, stderr.String())
	}
	stdout.Reset()
	if Main([]string{"show"}, nil, &stdout, &stderr) != 0 {
		t.Fatalf("show stderr %s", stderr.String())
	}
	text := stdout.String()
	for _, want := range []string{
		"origin.harness: zero",
		"origin.version: zero 0.9.0",
		"origin.sessionId: zero_test_1",
		"origin.cwd: /tmp/proj",
		"origin.resume: zero exec --resume zero_test_1",
		"path /tmp/proj",
		"token stays",
		"dfd1fe8a1b2c3d4e5f60718293a4b5c6d7e8f901",
		"[REDACTED]",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("show missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "9f3c1a7e") {
		t.Fatalf("secret leaked in show:\n%s", text)
	}
	stdout.Reset()
	if Main([]string{"show", "--harness", "hermes"}, nil, &stdout, &stderr) != 1 {
		t.Fatalf("hermes filter = %q", stdout.String())
	}
}

func TestSkipWithoutSessionWritesNothing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HANDOFF_DIR", dir)
	t.Setenv("HANDOFF_VERSION", "")
	stdin := strings.NewReader(`{"event":"sessionEnd","sessionId":"","cwd":"/tmp/proj","stopReason":"","turns":1}`)
	var stdout, stderr bytes.Buffer
	code := Main([]string{"save", "--harness", "zero", "--skip-without-session"}, stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("skip code %d stderr %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "no sessionId") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("wrote %d files", len(entries))
	}
	stdout.Reset()
	if Main([]string{"show"}, nil, &stdout, &stderr) != 1 {
		t.Fatalf("show = %q", stdout.String())
	}
	stderr.Reset()
	code = Main([]string{"save", "--harness", "zero", "--version", "zero 0.9.0"}, strings.NewReader(`{"event":"sessionEnd","sessionId":"","cwd":"/tmp/proj"}`), &stdout, &stderr)
	if code == 0 {
		t.Fatal("save without the flag accepted an empty sessionId")
	}
	if !strings.Contains(stderr.String(), "sessionId") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestSessionJSONUsesExplicitHarness(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HANDOFF_DIR", dir)
	var stdout, stderr bytes.Buffer
	stdin := strings.NewReader(`{"event":"sessionEnd","sessionId":"zero_20261009_1","cwd":"/tmp/proj","stopReason":"error","turns":1,"error":"provider"}`)
	code := Main([]string{"save", "--harness", "zero", "--version", "zero 0.9.0"}, stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("save code %d stderr %s", code, stderr.String())
	}
	stdout.Reset()
	if Main([]string{"show"}, nil, &stdout, &stderr) != 0 {
		t.Fatal(stderr.String())
	}
	text := stdout.String()
	if !strings.Contains(text, "origin.resume: zero exec --resume zero_20261009_1") {
		t.Fatalf("resume missing:\n%s", text)
	}
	if !strings.Contains(text, "stopReason=error") || !strings.Contains(text, "error=provider") {
		t.Fatalf("synthesized note missing:\n%s", text)
	}
}
