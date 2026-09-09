package store

import (
	"testing"

	"github.com/tcontardo/prettylogs/internal/record"
)

func rec(level, msg string) record.Record {
	return record.Record{Level: level, Message: msg, Raw: msg}
}

func TestAddAssignsIDsAndKeepsAllWhenUnfiltered(t *testing.T) {
	t.Parallel()
	s := New(10)
	s.Add(rec(record.LevelInfo, "a"))
	s.Add(rec(record.LevelError, "b"))
	got := s.Filtered()
	if len(got) != 2 {
		t.Fatalf("filtered len %d want 2", len(got))
	}
	if got[0].ID != 1 || got[1].ID != 2 {
		t.Fatalf("ids %d,%d", got[0].ID, got[1].ID)
	}
}

func TestSetLevelFiltersExistingAndNew(t *testing.T) {
	t.Parallel()
	s := New(10)
	s.Add(rec(record.LevelInfo, "ok"))
	s.Add(rec(record.LevelError, "boom"))
	s.SetLevel(record.LevelError)
	if got := s.Filtered(); len(got) != 1 || got[0].Message != "boom" {
		t.Fatalf("after level filter: %+v", got)
	}
	s.Add(rec(record.LevelWarn, "skip"))
	s.Add(rec(record.LevelError, "again"))
	got := s.Filtered()
	if len(got) != 2 {
		t.Fatalf("incremental filtered len %d want 2", len(got))
	}
	if got[1].Message != "again" {
		t.Fatalf("second match %q", got[1].Message)
	}
}

func TestSetSearchRegex(t *testing.T) {
	t.Parallel()
	s := New(10)
	s.Add(rec(record.LevelInfo, "alpha"))
	s.Add(rec(record.LevelInfo, "beta"))
	s.SetSearch("a.+a")
	got := s.Filtered()
	if len(got) != 1 || got[0].Message != "alpha" {
		t.Fatalf("regex filter: %+v", got)
	}
}

func TestInvalidRegexMatchesNothing(t *testing.T) {
	t.Parallel()
	s := New(10)
	s.Add(rec(record.LevelInfo, "hello"))
	s.SetSearch("(")
	if s.QueryError() == nil {
		t.Fatal("expected query error")
	}
	if len(s.Filtered()) != 0 {
		t.Fatal("invalid regex should match nothing")
	}
}

func TestMaxSizeDropsOldest(t *testing.T) {
	t.Parallel()
	s := New(2)
	s.Add(rec(record.LevelInfo, "one"))
	s.Add(rec(record.LevelInfo, "two"))
	s.Add(rec(record.LevelInfo, "three"))
	if s.Len() != 2 {
		t.Fatalf("len %d want 2", s.Len())
	}
	got := s.Filtered()
	if got[0].Message != "two" || got[1].Message != "three" {
		t.Fatalf("remaining %+v", []string{got[0].Message, got[1].Message})
	}
}

func TestCountsIgnoreFilter(t *testing.T) {
	t.Parallel()
	s := New(10)
	s.Add(rec(record.LevelError, "a"))
	s.Add(rec(record.LevelError, "b"))
	s.Add(rec(record.LevelWarn, "c"))
	s.SetLevel(record.LevelWarn)
	counts := s.Counts()
	if counts[record.LevelError] != 2 || counts[record.LevelWarn] != 1 {
		t.Fatalf("counts %+v", counts)
	}
}

func TestLevelAndSearchCombine(t *testing.T) {
	t.Parallel()
	s := New(10)
	s.Add(rec(record.LevelError, "db down"))
	s.Add(rec(record.LevelError, "auth fail"))
	s.Add(rec(record.LevelInfo, "db up"))
	s.SetLevel(record.LevelError)
	s.SetSearch("db")
	got := s.Filtered()
	if len(got) != 1 || got[0].Message != "db down" {
		t.Fatalf("combined filter: %+v", got)
	}
}
