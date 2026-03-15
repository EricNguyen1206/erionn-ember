package store

import (
	"reflect"
	"testing"
)

func TestListPushPopAndRange(t *testing.T) {
	s := New()

	length, err := s.LPush("jobs", []string{"b", "a"})
	if err != nil || length != 2 {
		t.Fatalf("unexpected lpush result: length=%d err=%v", length, err)
	}
	length, err = s.RPush("jobs", []string{"c"})
	if err != nil || length != 3 {
		t.Fatalf("unexpected rpush result: length=%d err=%v", length, err)
	}

	items, found, err := s.LRange("jobs", 0, 2)
	if err != nil || !found || !reflect.DeepEqual(items, []string{"a", "b", "c"}) {
		t.Fatalf("unexpected lrange result: items=%v found=%v err=%v", items, found, err)
	}
	items, found, err = s.LRange("jobs", -2, -1)
	if err != nil || !found || !reflect.DeepEqual(items, []string{"b", "c"}) {
		t.Fatalf("unexpected negative lrange result: items=%v found=%v err=%v", items, found, err)
	}

	item, found, err := s.LPop("jobs")
	if err != nil || !found || item != "a" {
		t.Fatalf("unexpected lpop result: item=%q found=%v err=%v", item, found, err)
	}
	item, found, err = s.RPop("jobs")
	if err != nil || !found || item != "c" {
		t.Fatalf("unexpected rpop result: item=%q found=%v err=%v", item, found, err)
	}

	_, found, err = s.LPop("missing")
	if err != nil || found {
		t.Fatalf("expected missing list pop to return found=false: found=%v err=%v", found, err)
	}
}

func TestListCommandsValidateInputAndType(t *testing.T) {
	s := New()

	if _, err := s.LPush("", []string{"a"}); err != ErrEmptyKey {
		t.Fatalf("got err %v, want %v", err, ErrEmptyKey)
	}
	if _, err := s.RPush("jobs", nil); err != ErrEmptyValues {
		t.Fatalf("got err %v, want %v", err, ErrEmptyValues)
	}

	s.entries["name"] = &Entry{Key: "name", Type: TypeString, Value: "ember"}
	if _, err := s.LPush("name", []string{"a"}); err != ErrWrongType {
		t.Fatalf("got err %v, want %v", err, ErrWrongType)
	}
	if _, _, err := s.LRange("name", 0, 1); err != ErrWrongType {
		t.Fatalf("got err %v, want %v", err, ErrWrongType)
	}
}
