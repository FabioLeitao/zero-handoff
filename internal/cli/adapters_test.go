package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func TestNoNetworkImports(t *testing.T) {
	root := moduleRoot(t)
	var bad []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && (info.Name() == ".git" || info.Name() == ".remember") {
			return filepath.SkipDir
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(body)
		for _, needle := range []string{"net/http", "net.Dial", "ListenAndServe"} {
			if strings.Contains(text, needle) {
				bad = append(bad, path+": "+needle)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) > 0 {
		t.Fatalf("network usage: %s", strings.Join(bad, "; "))
	}
}

func TestProjectHookDeclaresHarness(t *testing.T) {
	root := moduleRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "adapters/zero/hooks.project.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(`"--harness"`)) || !bytes.Contains(body, []byte(`"zero"`)) {
		t.Fatalf("project hook must pass --harness zero:\n%s", body)
	}
	if !bytes.Contains(body, []byte(`"id"`)) {
		t.Fatalf("project hook needs an id or zero drops the file:\n%s", body)
	}
	var manifest map[string]any
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatal(err)
	}
}

func TestPluginManifest(t *testing.T) {
	root := moduleRoot(t)
	dir := filepath.Join(root, "adapters/zero/plugin")
	body, err := os.ReadFile(filepath.Join(dir, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		SchemaVersion int `json:"schemaVersion"`
		Tools         []struct {
			Name    string `json:"name"`
			Command string `json:"command"`
		} `json:"tools"`
		Hooks []struct {
			Event   string `json:"event"`
			Command string `json:"command"`
		} `json:"hooks"`
		Skills []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != 1 {
		t.Fatalf("schemaVersion = %d", manifest.SchemaVersion)
	}
	if len(manifest.Tools) != 1 || manifest.Tools[0].Name != "recall_handoff" {
		t.Fatalf("tools = %+v", manifest.Tools)
	}
	if len(manifest.Hooks) != 1 || manifest.Hooks[0].Event != "sessionEnd" {
		t.Fatalf("hooks = %+v", manifest.Hooks)
	}
	if len(manifest.Skills) != 1 || manifest.Skills[0].Name != "remind" {
		t.Fatalf("skills = %+v", manifest.Skills)
	}
	skill, err := os.ReadFile(filepath.Join(dir, manifest.Skills[0].Path))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(skill, []byte("---\n")) {
		t.Fatal("SKILL.md is missing frontmatter")
	}
	hookBody, err := os.ReadFile(filepath.Join(dir, manifest.Hooks[0].Command))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(hookBody, []byte("../")) {
		t.Fatalf("plugin hook is not self-contained:\n%s", hookBody)
	}
	if !bytes.Contains(hookBody, []byte("--harness zero")) {
		t.Fatal("plugin hook must pass --harness zero")
	}
	for _, rel := range []string{
		manifest.Tools[0].Command,
		manifest.Hooks[0].Command,
		manifest.Skills[0].Path,
	} {
		path := filepath.Join(dir, rel)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(rel, ".sh") && info.Mode()&0o111 == 0 {
			t.Fatalf("%s is not executable", path)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "adapters/zero/commands/remind.md")); err != nil {
		t.Fatal(err)
	}
}

func TestSessionEndScriptEndToEnd(t *testing.T) {
	root := moduleRoot(t)
	binDir := t.TempDir()
	data := t.TempDir()
	cmd := exec.Command("go", "build", "-o", filepath.Join(binDir, "handoff"), "./cmd/handoff")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	script := filepath.Join(root, "adapters/zero/session-end.sh")
	run := func(t *testing.T, extra []string, stdin string) (int, string, string) {
		t.Helper()
		c := exec.Command(script, extra...)
		c.Stdin = strings.NewReader(stdin)
		var stdout, stderr bytes.Buffer
		c.Stdout = &stdout
		c.Stderr = &stderr
		c.Env = append(os.Environ(),
			"PATH="+binDir+":"+os.Getenv("PATH"),
			"HANDOFF_DIR="+data,
			"HANDOFF_VERSION=zero 0.9.0",
		)
		err := c.Run()
		code := 0
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			} else {
				t.Fatal(err)
			}
		}
		return code, stdout.String(), stderr.String()
	}

	show := exec.Command(filepath.Join(binDir, "handoff"), "show")
	show.Env = append(os.Environ(), "HANDOFF_DIR="+data)
	out, err := show.CombinedOutput()
	if err == nil {
		t.Fatalf("empty show succeeded: %s", out)
	}
	if !bytes.Contains(out, []byte("no handoff entry")) {
		t.Fatalf("negation output = %q", out)
	}

	payload := `{"event":"sessionEnd","sessionId":"zero_e2e_1","cwd":"/tmp/proj","stopReason":"error","turns":1}`
	code, _, stderr := run(t, nil, payload)
	if code != 0 {
		t.Fatalf("session-end code %d stderr %s", code, stderr)
	}
	show = exec.Command(filepath.Join(binDir, "handoff"), "show")
	show.Env = append(os.Environ(), "HANDOFF_DIR="+data)
	out, err = show.CombinedOutput()
	if err != nil {
		t.Fatalf("show: %v\n%s", err, out)
	}
	text := string(out)
	for _, want := range []string{
		"origin.harness: zero",
		"origin.sessionId: zero_e2e_1",
		"origin.resume: zero exec --resume zero_e2e_1",
		"stopReason=error",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q\n%s", want, text)
		}
	}

	recall := exec.Command(filepath.Join(root, "adapters/zero/plugin/tools/recall_handoff.sh"))
	recall.Env = append(os.Environ(), "PATH="+binDir, "HANDOFF_DIR="+data)
	recall.Stdin = strings.NewReader(`{"ignored":true}`)
	recalled, err := recall.CombinedOutput()
	if err != nil {
		t.Fatalf("recall: %v\n%s", err, recalled)
	}
	if !bytes.Contains(recalled, []byte("origin.resume: zero exec --resume zero_e2e_1")) {
		t.Fatalf("recall = %s", recalled)
	}
}
