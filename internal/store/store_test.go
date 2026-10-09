package store

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sample(at, note string) Entry {
	return Entry{
		Origin: Origin{
			Harness:   "zero",
			Version:   "zero 0.9.0",
			SessionID: "zero_test_1",
			Cwd:       "/tmp/proj",
			At:        at,
			Resume:    "zero exec --resume zero_test_1",
		},
		Note: note,
	}
}

func TestSaveRejectsMissingOrigin(t *testing.T) {
	dir := t.TempDir()
	_, err := (Store{Dir: dir}).Save(Entry{Note: "hello"})
	if err == nil {
		t.Fatal("expected missing origin to fail")
	}
	entries, _ := filepath.Glob(filepath.Join(dir, "*"))
	if len(entries) != 0 {
		t.Fatalf("wrote files on failure: %v", entries)
	}
}

func TestSaveRedactsAndIsAppendOnly(t *testing.T) {
	dir := t.TempDir()
	s := Store{Dir: dir}
	first, err := s.Save(sample("2026-10-09T12:00:00Z", "plain note"))
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Save(sample("2026-10-09T12:00:01Z", "token=supersecretvalue"))
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Fatal("second save reused the first file")
	}
	got, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, got) {
		t.Fatal("first entry was rewritten")
	}
	stored, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(stored, []byte("supersecretvalue")) {
		t.Fatalf("secret stored: %s", stored)
	}
}

func TestLinkDoesNotReplaceExistingFile(t *testing.T) {
	dir := t.TempDir()
	entry := sample("2026-10-09T12:00:00Z", "fresh")
	occupied := filepath.Join(dir, fileName(entry))
	if err := os.WriteFile(occupied, []byte("KEEP\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	saved, err := (Store{Dir: dir}).Save(entry)
	if err != nil {
		t.Fatal(err)
	}
	if saved == occupied {
		t.Fatal("save replaced the occupied name")
	}
	body, err := os.ReadFile(occupied)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "KEEP\n" {
		t.Fatalf("occupied file changed: %q", body)
	}
	if _, err := os.Stat(saved); err != nil {
		t.Fatal(err)
	}
}

func TestLatestParsesTimestampNotStringOrder(t *testing.T) {
	dir := t.TempDir()
	s := Store{Dir: dir}
	// "Z" sorts after "." so a string compare would pick the earlier instant.
	earlier := sample("2026-10-09T00:00:00Z", "earlier")
	later := sample("2026-10-09T00:00:00.1Z", "later")
	later.Origin.SessionID = "zero_test_2"
	later.Origin.Resume = "zero exec --resume zero_test_2"
	if _, err := s.Save(later); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(earlier); err != nil {
		t.Fatal(err)
	}
	got, err := s.Latest(Filter{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Note != "later" {
		t.Fatalf("latest note = %q, want later", got.Note)
	}
}

func TestLatestSkipsCorruptJSON(t *testing.T) {
	dir := t.TempDir()
	s := Store{Dir: dir}
	if _, err := s.Save(sample("2026-10-09T00:00:00Z", "good")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "000-corrupt.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	got, err := s.Latest(Filter{}, &warn)
	if err != nil {
		t.Fatal(err)
	}
	if got.Note != "good" {
		t.Fatalf("note = %q", got.Note)
	}
	if !strings.Contains(warn.String(), "000-corrupt.json") {
		t.Fatalf("warning = %q", warn.String())
	}
}

func TestShowFilterAndEmpty(t *testing.T) {
	dir := t.TempDir()
	s := Store{Dir: dir}
	if _, err := s.Latest(Filter{}, nil); err != ErrNone {
		t.Fatalf("empty dir err = %v", err)
	}
	if _, err := s.Save(sample("2026-10-09T00:00:00Z", "from zero")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Latest(Filter{Harness: "hermes"}, nil); err != ErrNone {
		t.Fatal("expected no hermes entry")
	}
	got, err := s.Latest(Filter{Cwd: "/tmp/proj"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(Format(got), "origin.harness: zero") {
		t.Fatalf("format = %s", Format(got))
	}
	if !strings.Contains(Format(got), "origin.resume: zero exec --resume zero_test_1") {
		t.Fatalf("format = %s", Format(got))
	}
}
