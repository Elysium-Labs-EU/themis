package fix

import (
	"strings"
	"testing"
)

func TestDirectiveValue(t *testing.T) {
	content := "Port 22\n#PermitRootLogin no\nPermitRootLogin yes\n"
	if got := DirectiveValue(content, "PermitRootLogin"); got != "yes" {
		t.Errorf("got %q, want %q", got, "yes")
	}
	if got := DirectiveValue(content, "PasswordAuthentication"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// TestDirectiveValueFirstWins reproduces F-023: OpenSSH's sshd uses the
// first occurrence of a global directive and ignores later duplicates, so
// DirectiveValue must return "yes" here, not the last line's "no" — the
// real running sshd is still wide open despite the later, ineffective
// line.
func TestDirectiveValueFirstWins(t *testing.T) {
	content := "PermitRootLogin yes\nPermitRootLogin no\n"
	if got := DirectiveValue(content, "PermitRootLogin"); got != "yes" {
		t.Errorf("got %q, want %q (sshd_config is first-match-wins)", got, "yes")
	}
}

// TestDirectiveValueFirstWinsAcrossMultipleMatchBlocks reproduces F-023
// against a config with several Match blocks, each repeating the
// directive: only the global section (before the first Match line)
// matters, and within it the first occurrence — not the last, and not
// anything inside a Match block — is authoritative.
func TestDirectiveValueFirstWinsAcrossMultipleMatchBlocks(t *testing.T) {
	content := "PermitRootLogin yes\nPermitRootLogin no\n" +
		"Match User admin\n    PermitRootLogin yes\n" +
		"Match Address 10.0.0.0/8\n    PermitRootLogin no\n"
	if got := DirectiveValue(content, "PermitRootLogin"); got != "yes" {
		t.Errorf("got %q, want %q (first global occurrence, ignoring all Match blocks)", got, "yes")
	}
}

func TestSetDirectiveAppendsWhenMissing(t *testing.T) {
	got := setDirective("Port 22\n", "PermitRootLogin", "no")
	want := "Port 22\nPermitRootLogin no"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSetDirectiveNoopWhenAlreadyCorrect(t *testing.T) {
	content := "Port 22\nPermitRootLogin no\n"
	got := setDirective(content, "PermitRootLogin", "no")
	if DirectiveValue(got, "PermitRootLogin") != "no" {
		t.Errorf("expected PermitRootLogin no to remain effective, got %q", got)
	}
}

func TestSetDirectiveCommentsOutConflicting(t *testing.T) {
	content := "PermitRootLogin yes\n"
	got := setDirective(content, "PermitRootLogin", "no")
	if DirectiveValue(got, "PermitRootLogin") != "no" {
		t.Errorf("expected PermitRootLogin no to be effective, got %q", got)
	}
	if got == content {
		t.Errorf("expected content to change")
	}
}

// The following reproduce the two scenarios from issue #15: a Match
// block scoping a directive must never be read as (or rewritten into)
// the global/general-case value.

func TestDirectiveValueIgnoresMatchBlockNarrowerException(t *testing.T) {
	// Global is wide open; a Match block narrows it for one subnet only.
	// The effective value for everyone else (what SSH-7408 cares about)
	// is still "yes" — DirectiveValue must not report "no" just because
	// a later, scoped line happens to say so.
	content := "PasswordAuthentication no\nPermitRootLogin yes\nMatch Address 10.0.0.0/8\n    PermitRootLogin no\n"
	if got := DirectiveValue(content, "PermitRootLogin"); got != "yes" {
		t.Errorf("got %q, want %q (Match block must not mask the insecure global default)", got, "yes")
	}
}

func TestDirectiveValueIgnoresMatchBlockException(t *testing.T) {
	// Global is already hardened; a Match block grants a deliberate
	// per-user exception. The global/general-case value is still "no".
	content := "PasswordAuthentication no\nPermitRootLogin no\nMatch User admin\n    PermitRootLogin yes\n"
	if got := DirectiveValue(content, "PermitRootLogin"); got != "no" {
		t.Errorf("got %q, want %q", got, "no")
	}
}

func TestDirectiveValueMatchLineIsCaseInsensitive(t *testing.T) {
	content := "PermitRootLogin yes\nmatch Address 10.0.0.0/8\n    PermitRootLogin no\n"
	if got := DirectiveValue(content, "PermitRootLogin"); got != "yes" {
		t.Errorf("got %q, want %q (lowercase match keyword must still open a Match block)", got, "yes")
	}
}

func TestSetDirectiveFixesGlobalWithoutTouchingNarrowerMatchException(t *testing.T) {
	// Issue #15 scenario A: global PermitRootLogin yes, Match Address
	// block already says no for one subnet. Applying the fix must
	// comment out the insecure global line and leave the Match block's
	// stricter line completely alone (nothing to fix there).
	content := "PasswordAuthentication no\nPermitRootLogin yes\nMatch Address 10.0.0.0/8\n    PermitRootLogin no\n"
	got := setDirective(content, "PermitRootLogin", "no")

	if DirectiveValue(got, "PermitRootLogin") != "no" {
		t.Errorf("expected global PermitRootLogin no to be effective, got:\n%s", got)
	}
	if !strings.Contains(got, "#PermitRootLogin yes") {
		t.Errorf("expected insecure global line to be commented out, got:\n%s", got)
	}
	if !strings.Contains(got, "Match Address 10.0.0.0/8\n    PermitRootLogin no") {
		t.Errorf("expected Match block to be left byte-for-byte untouched, got:\n%s", got)
	}
}

func TestSetDirectiveDoesNotDestroyMatchBlockException(t *testing.T) {
	// Issue #15 scenario B: global already hardened, Match User admin
	// grants a deliberate break-glass exception. Since the global value
	// is already correct, setDirective must be a no-op — in particular
	// it must never comment out the Match-scoped "PermitRootLogin yes".
	content := "PasswordAuthentication no\nPermitRootLogin no\nMatch User admin\n    PermitRootLogin yes\n"
	got := setDirective(content, "PermitRootLogin", "no")

	if strings.Contains(got, "#") {
		t.Errorf("expected no line to be commented out (global already satisfied), got:\n%s", got)
	}
	if !strings.Contains(got, "Match User admin\n    PermitRootLogin yes") {
		t.Errorf("expected admin's Match-scoped override to survive untouched, got:\n%s", got)
	}
}

func TestSetDirectiveAppendsBeforeMatchBlockWhenMissingGlobally(t *testing.T) {
	// key has no global setting at all, only a Match-scoped one. The
	// new global directive must be inserted before the Match block, not
	// after (which would make it Match-scoped by accident), and must not
	// disturb the existing Match-scoped line.
	content := "Port 22\nMatch User admin\n    PermitRootLogin yes\n"
	got := setDirective(content, "PermitRootLogin", "no")

	if DirectiveValue(got, "PermitRootLogin") != "no" {
		t.Errorf("expected global PermitRootLogin no to be effective, got:\n%s", got)
	}
	matchIdx := strings.Index(got, "Match User admin")
	directiveIdx := strings.Index(got, "PermitRootLogin no")
	if matchIdx < 0 || directiveIdx < 0 || directiveIdx > matchIdx {
		t.Errorf("expected inserted directive before the Match block, got:\n%s", got)
	}
	if !strings.Contains(got, "Match User admin\n    PermitRootLogin yes") {
		t.Errorf("expected existing Match-scoped line to survive untouched, got:\n%s", got)
	}
}

func TestDirectiveInMatchBlock(t *testing.T) {
	if directiveInMatchBlock("PermitRootLogin no\n", "PermitRootLogin") {
		t.Error("expected false: no Match block at all")
	}
	if directiveInMatchBlock("PermitRootLogin no\nMatch User admin\n    PasswordAuthentication yes\n", "PermitRootLogin") {
		t.Error("expected false: Match block exists but doesn't redefine this key")
	}
	if !directiveInMatchBlock("PermitRootLogin no\nMatch User admin\n    PermitRootLogin yes\n", "PermitRootLogin") {
		t.Error("expected true: Match block redefines this key")
	}
}
