package store

import (
	"reflect"
	"testing"
)

func TestSetAddRemoveMembersAndMembership(t *testing.T) {
	s := New()

	added, err := s.SAdd("tags", []string{"go", "grpc", "go"})
	if err != nil || added != 2 {
		t.Fatalf("unexpected sadd result: added=%d err=%v", added, err)
	}

	members, found, err := s.SMembers("tags")
	if err != nil || !found || !reflect.DeepEqual(members, []string{"go", "grpc"}) {
		t.Fatalf("unexpected smembers result: members=%v found=%v err=%v", members, found, err)
	}

	isMember, err := s.SIsMember("tags", "grpc")
	if err != nil || !isMember {
		t.Fatalf("unexpected sismember result: isMember=%v err=%v", isMember, err)
	}

	removed, err := s.SRem("tags", []string{"go"})
	if err != nil || removed != 1 {
		t.Fatalf("unexpected srem result: removed=%d err=%v", removed, err)
	}

	isMember, err = s.SIsMember("missing", "grpc")
	if err != nil || isMember {
		t.Fatalf("expected missing set member lookup to be false: isMember=%v err=%v", isMember, err)
	}
}

func TestSetCommandsValidateInputAndType(t *testing.T) {
	s := New()

	if _, err := s.SAdd("", []string{"go"}); err != ErrEmptyKey {
		t.Fatalf("got err %v, want %v", err, ErrEmptyKey)
	}
	if _, err := s.SAdd("tags", nil); err != ErrEmptyValues {
		t.Fatalf("got err %v, want %v", err, ErrEmptyValues)
	}

	s.entries["name"] = &Entry{Key: "name", Type: TypeString, Value: "ember"}
	if _, err := s.SAdd("name", []string{"go"}); err != ErrWrongType {
		t.Fatalf("got err %v, want %v", err, ErrWrongType)
	}
	if _, _, err := s.SMembers("name"); err != ErrWrongType {
		t.Fatalf("got err %v, want %v", err, ErrWrongType)
	}
}
