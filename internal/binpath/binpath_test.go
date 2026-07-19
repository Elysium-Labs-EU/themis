package binpath

import (
	"strings"
	"testing"
)

func TestResolveInFindsFirstMatchingDir(t *testing.T) {
	exists := func(p string) bool { return p == "/sbin/systemctl" }

	got, err := resolveIn("systemctl", []string{"/usr/sbin", "/sbin"}, exists)
	if err != nil {
		t.Fatalf("resolveIn: %v", err)
	}
	if got != "/sbin/systemctl" {
		t.Errorf("got %q, want %q", got, "/sbin/systemctl")
	}
}

func TestResolveInPrefersEarlierDir(t *testing.T) {
	exists := func(p string) bool {
		return p == "/usr/sbin/lynis" || p == "/sbin/lynis"
	}

	got, err := resolveIn("lynis", []string{"/usr/sbin", "/sbin"}, exists)
	if err != nil {
		t.Fatalf("resolveIn: %v", err)
	}
	if got != "/usr/sbin/lynis" {
		t.Errorf("got %q, want %q", got, "/usr/sbin/lynis")
	}
}

func TestResolveInErrorsWhenNowhereFound(t *testing.T) {
	exists := func(string) bool { return false }

	_, err := resolveIn("nope", []string{"/usr/sbin", "/sbin"}, exists)
	if err == nil {
		t.Fatal("expected an error when name is not in any trusted dir")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %q should mention the command name", err.Error())
	}
}

func TestResolveFindsRealBinary(t *testing.T) {
	// "true" is a standard coreutils binary present at /usr/bin/true or
	// /bin/true on any Linux or macOS dev/CI box — exercises the real
	// TrustedDirs + os.Stat path, not just the injected fake.
	got, err := Resolve("true")
	if err != nil {
		t.Fatalf("Resolve(true): %v", err)
	}
	if got == "" {
		t.Error("expected a non-empty path")
	}
}

func TestResolveErrorsForUnknownCommand(t *testing.T) {
	if _, err := Resolve("definitely-not-a-real-binary-xyz123"); err == nil {
		t.Fatal("expected an error for a nonexistent command")
	}
}

func TestEnvironOverridesPoisonedPath(t *testing.T) {
	base := []string{"PATH=/opt/evil:/usr/bin", "LANG=C", "HOME=/root"}

	got := Environ(base)

	var path string
	seen := map[string]string{}
	for _, kv := range got {
		k, v, _ := strings.Cut(kv, "=")
		seen[k] = v
		if k == "PATH" {
			path = v
		}
	}
	if path != SanitizedPath {
		t.Errorf("PATH = %q, want %q", path, SanitizedPath)
	}
	if strings.Contains(path, "/opt/evil") {
		t.Errorf("PATH %q still contains the poisoned entry", path)
	}
	if seen["LANG"] != "C" || seen["HOME"] != "/root" {
		t.Errorf("Environ dropped a non-PATH variable: %v", seen)
	}
}

func TestEnvironAddsPathWhenAbsent(t *testing.T) {
	got := Environ([]string{"LANG=C"})

	found := false
	for _, kv := range got {
		if kv == "PATH="+SanitizedPath {
			found = true
		}
	}
	if !found {
		t.Errorf("expected PATH=%s in %v", SanitizedPath, got)
	}
}
