package store

import (
	"sort"
	"time"
)

// zsetEntry is the in-memory representation of a sorted set: member → score.
type zsetEntry map[string]float64

func (s *Store) ZAdd(key string, members map[string]float64) (int, error) {
	if key == "" {
		return 0, ErrEmptyKey
	}
	if len(members) == 0 {
		return 0, ErrEmptyValues
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	zset, entry, err := s.getOrCreateZSetLocked(key)
	if err != nil {
		return 0, err
	}

	added := 0
	for member, score := range members {
		if _, exists := zset[member]; !exists {
			added++
		}
		zset[member] = score
	}
	entry.UpdatedAt = time.Now()
	return added, nil
}

func (s *Store) ZCard(key string) (int, error) {
	if key == "" {
		return 0, ErrEmptyKey
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	zset, _, found, err := s.getZSetRLocked(key)
	if err != nil || !found {
		return 0, err
	}
	return len(zset), nil
}

// ZRange returns members ordered by score ascending, using 0-based index range.
// Negative indices wrap from the end (e.g. -1 = last element).
func (s *Store) ZRange(key string, start, stop int) ([]string, bool, error) {
	if key == "" {
		return nil, false, ErrEmptyKey
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	zset, _, found, err := s.getZSetRLocked(key)
	if err != nil || !found {
		return nil, false, err
	}

	// Build sorted slice by score
	type scoredMember struct {
		member string
		score  float64
	}
	sorted := make([]scoredMember, 0, len(zset))
	for member, score := range zset {
		sorted = append(sorted, scoredMember{member, score})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].score != sorted[j].score {
			return sorted[i].score < sorted[j].score
		}
		return sorted[i].member < sorted[j].member
	})

	n := len(sorted)
	// Normalize negative indices
	if start < 0 {
		start = n + start
	}
	if stop < 0 {
		stop = n + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= n {
		stop = n - 1
	}
	if start > stop {
		return []string{}, true, nil
	}

	result := make([]string, 0, stop-start+1)
	for _, sm := range sorted[start : stop+1] {
		result = append(result, sm.member)
	}
	return result, true, nil
}

// ZRemRangeByScore removes all members with score in [min, max] inclusive.
// Returns the number of removed members.
func (s *Store) ZRemRangeByScore(key string, min, max float64) (int, error) {
	if key == "" {
		return 0, ErrEmptyKey
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	zset, entry, found, err := s.getZSetLocked(key)
	if err != nil || !found {
		return 0, err
	}

	removed := 0
	for member, score := range zset {
		if score >= min && score <= max {
			delete(zset, member)
			removed++
		}
	}
	if removed > 0 {
		entry.UpdatedAt = time.Now()
	}
	if len(zset) == 0 {
		delete(s.entries, key)
	}
	return removed, nil
}

// --- internal helpers ---

func (s *Store) getOrCreateZSetLocked(key string) (zsetEntry, *Entry, error) {
	if entry, ok := s.entries[key]; ok {
		if s.pruneExpiredLocked(key, entry) {
			return s.createZSetEntryLocked(key)
		}
		if entry.Type != TypeZSet {
			return nil, nil, ErrWrongType
		}
		zset, ok := entry.Value.(zsetEntry)
		if !ok {
			return nil, nil, ErrInvalidValue
		}
		return zset, entry, nil
	}
	return s.createZSetEntryLocked(key)
}

func (s *Store) createZSetEntryLocked(key string) (zsetEntry, *Entry, error) {
	now := time.Now()
	zset := make(zsetEntry)
	entry := &Entry{
		Key:       key,
		Type:      TypeZSet,
		Value:     zset,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.entries[key] = entry
	return zset, entry, nil
}

func (s *Store) getZSetLocked(key string) (zsetEntry, *Entry, bool, error) {
	entry, ok := s.entries[key]
	if !ok || s.pruneExpiredLocked(key, entry) {
		return nil, nil, false, nil
	}
	if entry.Type != TypeZSet {
		return nil, nil, false, ErrWrongType
	}
	zset, ok := entry.Value.(zsetEntry)
	if !ok {
		return nil, nil, false, ErrInvalidValue
	}
	return zset, entry, true, nil
}

func (s *Store) getZSetRLocked(key string) (zsetEntry, *Entry, bool, error) {
	entry, ok := s.entries[key]
	if !ok || s.isExpired(entry) {
		return nil, nil, false, nil
	}
	if entry.Type != TypeZSet {
		return nil, nil, false, ErrWrongType
	}
	zset, ok := entry.Value.(zsetEntry)
	if !ok {
		return nil, nil, false, ErrInvalidValue
	}
	return zset, entry, true, nil
}
