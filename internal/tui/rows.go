package tui

import (
	"regexp"

	"github.com/tcontardo/prettylogs/internal/record"
)

const groupThreshold = 4

// warnKeyword and errorKeyword are case-insensitive, word-boundary safety
// nets used to veto folding a record whose raw text suggests it's actually
// more severe than its assigned Level — guarding against parser
// misclassification. The veto is level-relative: a WARN record containing
// "warn"/"warning" is not suspicious (that's its own level), but one
// containing "error"/"exception"/"fail" is, since that suggests it should
// really have been classified ERROR.
var (
	warnKeyword  = regexp.MustCompile(`(?i)\bwarn(?:ing)?\b`)
	errorKeyword = regexp.MustCompile(`(?i)\b(error|exception|fail(?:ed|ure)?)\b`)
)

// foldable reports whether rec may be folded into a collapsed group row.
// INFO/DEBUG/WARN are eligible; ERROR always renders standalone. INFO/DEBUG
// are excluded by either warn- or error-ish wording in rec.Raw; WARN is
// excluded only by error-ish wording, not by its own "warn" wording.
func foldable(rec *record.Record) bool {
	switch rec.Level {
	case record.LevelDebug, record.LevelInfo:
		return !warnKeyword.MatchString(rec.Raw) && !errorKeyword.MatchString(rec.Raw)
	case record.LevelWarn:
		return !errorKeyword.MatchString(rec.Raw)
	default:
		return false
	}
}

type rowKind int

const (
	rowRecord rowKind = iota
	rowGroup
)

// row is one renderable list item: either a single record, or a run of
// >= groupThreshold consecutive same-Level records collapsed together.
type row struct {
	kind  rowKind
	rec   *record.Record   // set when kind == rowRecord
	group []*record.Record // set when kind == rowGroup, len >= groupThreshold
}

// key is the identity used against Model.expanded. Individual rows key off
// their own record ID (identical to the existing per-record detail-expand
// feature); group rows key off their first member's ID. A record folded
// into a group is never simultaneously rendered as an individual row, so no
// key collisions occur within one rows() computation.
func (r row) key() uint64 {
	if r.kind == rowGroup {
		return r.group[0].ID
	}
	return r.rec.ID
}

// buildRows is a pure fold over entries (already level/search-filtered, in
// store order): walks maximal runs of consecutive equal Level and turns
// each run of length >= threshold into one rowGroup, leaving shorter runs
// as individual rowRecord rows.
func buildRows(entries []*record.Record, threshold int) []row {
	rows := make([]row, 0, len(entries))
	i := 0
	for i < len(entries) {
		if !foldable(entries[i]) {
			rows = append(rows, row{kind: rowRecord, rec: entries[i]})
			i++
			continue
		}
		j := i + 1
		for j < len(entries) && entries[j].Level == entries[i].Level && foldable(entries[j]) {
			j++
		}
		if j-i >= threshold {
			rows = append(rows, row{kind: rowGroup, group: entries[i:j:j]})
		} else {
			for k := i; k < j; k++ {
				rows = append(rows, row{kind: rowRecord, rec: entries[k]})
			}
		}
		i = j
	}
	return rows
}

// rows returns the current renderable rows, folding long same-level runs
// into groups. Used everywhere instead of m.store.Filtered() for indexing.
func (m Model) rows() []row {
	return buildRows(m.store.Filtered(), groupThreshold)
}
