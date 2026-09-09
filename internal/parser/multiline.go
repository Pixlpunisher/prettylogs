package parser

import (
	"strings"

	"github.com/tcontardo/prettylogs/internal/record"
)

func IsContinuation(line string) bool {
	if line == "" {
		return false
	}
	if line[0] == ' ' || line[0] == '\t' {
		return true
	}
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "at ") || strings.HasPrefix(trimmed, "Caused by:")
}

type Assembler struct {
	parser  Parser
	pending *record.Record
}

func NewAssembler(p Parser) *Assembler {
	return &Assembler{parser: p}
}

func (a *Assembler) Feed(line string) []record.Record {
	if a.pending != nil && IsContinuation(line) {
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
