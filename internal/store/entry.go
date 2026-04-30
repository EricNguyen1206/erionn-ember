package store

import "time"

type EntryType string

const (
	TypeNone   EntryType = "none"
	TypeString EntryType = "string"
	TypeHash   EntryType = "hash"
	TypeList   EntryType = "list"
	TypeSet    EntryType = "set"
	TypeZSet   EntryType = "zset"
)

type Entry struct {
	Key       string
	Type      EntryType
	Value     any
	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Stats struct {
	TotalKeys  int64
	StringKeys int64
	HashKeys   int64
	ListKeys   int64
	SetKeys    int64
	ZSetKeys   int64
}
