package store

import (
	"sync"
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

func TestStringIncrFromMissing(t *testing.T) {
	s := New()
	num, err := s.IncrString("counter")
	if err != nil || num != 1 {
		t.Fatalf("got num=%d err=%v, want 1 nil", num, err)
	}
	value, found, _ := s.GetString("counter")
	if !found || value != "1" {
		t.Fatalf("got value=%q found=%v", value, found)
	}
}

func TestStringIncrFromExisting(t *testing.T) {
	s := New()
	_ = s.SetString("counter", "10", 0)
	num, err := s.IncrString("counter")
	if err != nil || num != 11 {
		t.Fatalf("got num=%d err=%v, want 11 nil", num, err)
	}
}

func TestStringIncrWrongType(t *testing.T) {
	s := New()
	s.entries["myset"] = &Entry{Key: "myset", Type: TypeSet, Value: map[string]struct{}{}}
	_, err := s.IncrString("myset")
	if err != ErrWrongType {
		t.Fatalf("got err %v, want %v", err, ErrWrongType)
	}
}

func TestStringIncrNonNumeric(t *testing.T) {
	s := New()
	_ = s.SetString("text", "hello", 0)
	_, err := s.IncrString("text")
	if err != ErrNotInteger {
		t.Fatalf("got err %v, want %v", err, ErrNotInteger)
	}
}

func TestStringIncrConcurrent(t *testing.T) {
	s := New()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.IncrString("counter")
		}()
	}
	wg.Wait()
	value, found, _ := s.GetString("counter")
	if !found || value != "100" {
		t.Fatalf("got value=%q found=%v, want 100", value, found)
	}
}
