package parser

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/tcontardo/prettylogs/internal/record"
)

// IsContinuation reports whether line looks like it continues a stack trace
// started by a record at prevLevel, rather than a new independent log entry.
// Indentation alone isn't enough: tools like nx/npm indent whole subprocess
// output streams for cosmetic grouping, which would otherwise glue unrelated
// INFO lines onto whatever record happened to come first.
func IsContinuation(prevLevel, line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "at ") || strings.HasPrefix(trimmed, "Caused by:") || strings.HasPrefix(trimmed, "... ") {
		return true
	}
	if prevLevel != record.LevelError && prevLevel != record.LevelWarn {
		return false
	}
	return line[0] == ' ' || line[0] == '\t'
}

type Assembler struct {
	parser  Parser
	pending *record.Record
}

func NewAssembler(p Parser) *Assembler {
	return &Assembler{parser: p}
}

func (a *Assembler) Feed(line string) []record.Record {
	line = ansi.Strip(line)
	if strings.TrimSpace(line) == "" {
		return nil
	}
	if a.pending != nil && IsContinuation(a.pending.Level, line) {
		a.pending.Raw += "\n" + line
		if a.pending.Message != "" {
			a.pending.Message += "\n" + line
		} else {
			a.pending.Message = line
		}
		return nil
	}
	var out []record.Record
	if a.pending != nil {
		out = append(out, *a.pending)
		a.pending = nil
	}
	rec := ParseLine(a.parser, line)
	a.pending = &rec
	return out
}

func (a *Assembler) Flush() []record.Record {
	if a.pending == nil {
		return nil
	}
	rec := *a.pending
	a.pending = nil
	return []record.Record{rec}
}
