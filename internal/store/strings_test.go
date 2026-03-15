package store

import (
	"testing"
	"time"
)

func TestStringSetGetAndOverwrite(t *testing.T) {
	s := New()

	if err := s.SetString("name", "ember", 0); err != nil {
		t.Fatalf("SetString: %v", err)
	}
	value, found, err := s.GetString("name")
	if err != nil || !found || value != "ember" {
		t.Fatalf("unexpected get result: value=%q found=%v err=%v", value, found, err)
	}
	if err := s.SetString("name", "cache", time.Second); err != nil {
		t.Fatalf("SetString overwrite: %v", err)
	}
	value, found, err = s.GetString("name")
	if err != nil || !found || value != "cache" {
		t.Fatalf("unexpected overwrite result: value=%q found=%v err=%v", value, found, err)
	}

	ttl, hasTTL, found := s.TTL("name")
	if !found || !hasTTL || ttl <= 0 {
		t.Fatalf("unexpected ttl state after overwrite: ttl=%v hasTTL=%v found=%v", ttl, hasTTL, found)
	}
}

func TestStringGetReturnsWrongType(t *testing.T) {
	s := New()
	s.entries["letters"] = &Entry{Key: "letters", Type: TypeSet, Value: map[string]struct{}{"a": {}}}

	_, _, err := s.GetString("letters")
	if err != ErrWrongType {
		t.Fatalf("got err %v, want %v", err, ErrWrongType)
	}
}

func TestStringSetRejectsEmptyKey(t *testing.T) {
	s := New()

	if err := s.SetString("", "value", 0); err != ErrEmptyKey {
		t.Fatalf("got err %v, want %v", err, ErrEmptyKey)
	}
}

func TestStringGetRejectsEmptyKey(t *testing.T) {
	s := New()

	_, _, err := s.GetString("")
	if err != ErrEmptyKey {
		t.Fatalf("got err %v, want %v", err, ErrEmptyKey)
	}
}

func TestStringGetRejectsInvalidStoredValue(t *testing.T) {
	s := New()
	s.entries["broken"] = &Entry{Key: "broken", Type: TypeString, Value: 42}

	_, _, err := s.GetString("broken")
	if err != ErrInvalidValue {
		t.Fatalf("got err %v, want %v", err, ErrInvalidValue)
	}
}
