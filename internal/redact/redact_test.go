package redact

import (
	"strings"
	"testing"
)

func TestNoteRedactsSecretShapes(t *testing.T) {
	pem := "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXkBAAAA\n-----END OPENSSH PRIVATE KEY-----"
	input := strings.Join([]string{
		"sk-test-ZH-EVAL-9f3c1a7e-TOKEN",
		"sk-ant-api03-abcdefghijklmnopqrst",
		"sk-or-v1-abcdefghijklmnopqrst",
		"token=supersecretvalue",
		"xai-ABCDEFGHIJKLMNOP",
		"xoxb-1234567890-abcdefghij",
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N",
		"ghp_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"AKIAIOSFODNN7EXAMPLE",
		pem,
	}, "\n")
	got := Note(input)
	for _, secret := range []string{
		"9f3c1a7e",
		"sk-ant-api03-abcdefghijklmnopqrst",
		"sk-or-v1-abcdefghijklmnopqrst",
		"supersecretvalue",
		"xai-ABCDEFGHIJKLMNOP",
		"xoxb-1234567890-abcdefghij",
		"eyJzdWIiOiIxMjM0NTY3ODkw",
		"ghp_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"AKIAIOSFODNN7EXAMPLE",
		"b3BlbnNzaC1rZXkBAAAA",
		"BEGIN OPENSSH PRIVATE KEY",
	} {
		if strings.Contains(got, secret) {
			t.Errorf("redacted note still contains %q\n%s", secret, got)
		}
	}
	if !strings.Contains(got, Replacement) {
		t.Fatalf("expected %s in %q", Replacement, got)
	}
}

func TestNoteLeavesOrdinaryText(t *testing.T) {
	const input = "session ended cleanly\n" +
		"the word token appears here\n" +
		"path /home/leitao/.local/share/handoff\n" +
		"commit dfd1fe8a1b2c3d4e5f60718293a4b5c6d7e8f901\n"
	got := Note(input)
	if got != input {
		t.Fatalf("ordinary text changed\n got: %q\nwant: %q", got, input)
	}
}
