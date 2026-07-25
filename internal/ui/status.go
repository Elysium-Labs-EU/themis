package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// FixStatus is one of the semantic states check, plan, apply, and rollback
// report about a single fix. Every command maps its own outcome onto one of
// these before rendering, so the same state gets the same color (and, for
// FixLabel, the same padded width) no matter which command printed it —
// instead of each command hand-picking a style and wording independently.
type FixStatus int

const (
	// StatusSatisfied: the fix is already in place — no action needed or taken.
	StatusSatisfied FixStatus = iota
	// StatusPending: the fix is not yet satisfied and would act on the system.
	StatusPending
	// StatusChanged: apply or rollback acted on the system, and it succeeded.
	StatusChanged
	// StatusWarned: an action was withheld pending operator review (a Warn
	// hook fired, drift was detected, or the fix is no longer registered).
	StatusWarned
	// StatusFailed: an action was attempted and failed.
	StatusFailed
)

var fixStatusStyle = map[FixStatus]lipgloss.Style{
	StatusSatisfied: LabelSuccess,
	StatusPending:   LabelWarning,
	StatusChanged:   LabelSuccess,
	StatusWarned:    LabelWarning,
	StatusFailed:    LabelError,
}

// fixLabelWidth is the padded width every FixLabel renders at — the length
// of "[reverted]", the longest label word in use — so bracketed labels from
// plan, apply, and rollback line up in the same terminal column even when
// run back to back in one session.
const fixLabelWidth = len("[reverted]")

// FixLabel renders a bracketed status word (e.g. "[ok]", "[applied]") in
// the style for state, padded to a fixed column width. Used by plan, apply,
// and rollback, whose output is one status line per fix.
func FixLabel(word string, state FixStatus) string {
	return fixStatusStyle[state].Render(fmt.Sprintf("%-*s", fixLabelWidth, word))
}

// FixIcon renders a compact inline status marker (e.g. "✓ fixed") in the
// style for state, unpadded. Used by check, whose output places a status
// marker inline in prose or a table cell rather than at the start of its
// own line, where FixLabel's fixed padding would just add stray spaces.
func FixIcon(word string, state FixStatus) string {
	return fixStatusStyle[state].Render(word)
}
