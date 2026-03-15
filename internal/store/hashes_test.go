package store

import "testing"

func TestHashSetGetGetAllAndDelete(t *testing.T) {
	s := New()

	added, err := s.HSet("user:1", map[string]string{"name": "eric", "role": "owner"})
	if err != nil || added != 2 {
		t.Fatalf("unexpected hset result: added=%d err=%v", added, err)
	}
	added, err = s.HSet("user:1", map[string]string{"role": "admin", "team": "cache"})
	if err != nil || added != 1 {
		t.Fatalf("unexpected second hset result: added=%d err=%v", added, err)
	}

	value, found, err := s.HGet("user:1", "name")
	if err != nil || !found || value != "eric" {
		t.Fatalf("unexpected hget result: value=%q found=%v err=%v", value, found, err)
	}

	fields, found, err := s.HGetAll("user:1")
	if err != nil || !found || len(fields) != 3 || fields["role"] != "admin" {
		t.Fatalf("unexpected hgetall result: fields=%v found=%v err=%v", fields, found, err)
	}

	removed, err := s.HDel("user:1", []string{"role"})
	if err != nil || removed != 1 {
		t.Fatalf("unexpected hdel result: removed=%d err=%v", removed, err)
	}

	_, found, err = s.HGet("user:1", "role")
	if err != nil || found {
		t.Fatalf("expected deleted field to be missing: found=%v err=%v", found, err)
	}
}

func TestHashCommandsValidateInputAndType(t *testing.T) {
	s := New()

	if _, err := s.HSet("", map[string]string{"name": "eric"}); err != ErrEmptyKey {
		t.Fatalf("got err %v, want %v", err, ErrEmptyKey)
	}
	if _, err := s.HSet("user:1", nil); err != ErrEmptyFieldSet {
		t.Fatalf("got err %v, want %v", err, ErrEmptyFieldSet)
	}

	s.entries["name"] = &Entry{Key: "name", Type: TypeString, Value: "ember"}
	if _, _, err := s.HGet("name", "field"); err != ErrWrongType {
		t.Fatalf("got err %v, want %v", err, ErrWrongType)
	}
}
