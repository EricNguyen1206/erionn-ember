package store

import (
	"sync"
	"time"
)

type Store struct {
	mu      sync.Mutex
	entries map[string]*Entry
}

func New() *Store {
	return &Store{entries: make(map[string]*Entry)}
}

func (s *Store) Del(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[key]
	if !ok || s.pruneExpiredLocked(key, entry) {
		return false
	}

	delete(s.entries, key)
	return true
}

func (s *Store) Exists(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[key]
	if !ok || s.pruneExpiredLocked(key, entry) {
		return false
	}

	return true
}

func (s *Store) Type(key string) EntryType {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[key]
	if !ok || s.pruneExpiredLocked(key, entry) {
		return TypeNone
	}

	return entry.Type
}

func (s *Store) Expire(key string, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[key]
	if !ok || s.pruneExpiredLocked(key, entry) {
		return false
	}

	now := time.Now()
	entry.UpdatedAt = now
	entry.ExpiresAt = now.Add(ttl)
	return true
}

func (s *Store) TTL(key string) (time.Duration, bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[key]
	if !ok || s.pruneExpiredLocked(key, entry) {
		return 0, false, false
	}
	if entry.ExpiresAt.IsZero() {
		return 0, false, true
	}

	ttl := time.Until(entry.ExpiresAt)
	if ttl <= 0 {
		delete(s.entries, key)
		return 0, false, false
	}

	return ttl, true, true
}

func (s *Store) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats := Stats{}
	for key, entry := range s.entries {
		if s.pruneExpiredLocked(key, entry) {
			continue
		}

		stats.TotalKeys++
		switch entry.Type {
		case TypeString:
			stats.StringKeys++
		case TypeHash:
			stats.HashKeys++
		case TypeList:
			stats.ListKeys++
		case TypeSet:
			stats.SetKeys++
		}
	}

	return stats
}

func (s *Store) pruneExpiredLocked(key string, entry *Entry) bool {
	if entry == nil || entry.ExpiresAt.IsZero() || time.Now().Before(entry.ExpiresAt) {
		return false
	}

	delete(s.entries, key)
	return true
}
