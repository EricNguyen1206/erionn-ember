package store

import (
	"sort"
	"time"
)

func (s *Store) SAdd(key string, members []string) (int, error) {
	if key == "" {
		return 0, ErrEmptyKey
	}
	if len(members) == 0 {
		return 0, ErrEmptyValues
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	set, entry, err := s.getOrCreateSetLocked(key)
	if err != nil {
		return 0, err
	}

	added := 0
	for _, member := range members {
		if _, exists := set[member]; !exists {
			set[member] = struct{}{}
			added++
		}
	}
	entry.UpdatedAt = time.Now()
	return added, nil
}

func (s *Store) SRem(key string, members []string) (int, error) {
	if key == "" {
		return 0, ErrEmptyKey
	}
	if len(members) == 0 {
		return 0, ErrEmptyValues
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	set, entry, found, err := s.getSetLocked(key)
	if err != nil || !found {
		return 0, err
	}

	removed := 0
	for _, member := range members {
		if _, exists := set[member]; exists {
			delete(set, member)
			removed++
		}
	}
	if len(set) == 0 {
		delete(s.entries, key)
		return removed, nil
	}
	if removed > 0 {
		entry.UpdatedAt = time.Now()
	}
	return removed, nil
}

func (s *Store) SMembers(key string) ([]string, bool, error) {
	if key == "" {
		return nil, false, ErrEmptyKey
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	set, _, found, err := s.getSetRLocked(key)
	if err != nil || !found {
		return nil, false, err
	}

	members := make([]string, 0, len(set))
	for member := range set {
		members = append(members, member)
	}
	sort.Strings(members)
	return members, true, nil
}

func (s *Store) SIsMember(key, member string) (bool, error) {
	if key == "" {
		return false, ErrEmptyKey
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	set, _, found, err := s.getSetRLocked(key)
	if err != nil || !found {
		return false, err
	}

	_, exists := set[member]
	return exists, nil
}

func (s *Store) getOrCreateSetLocked(key string) (map[string]struct{}, *Entry, error) {
	if entry, ok := s.entries[key]; ok {
		if s.pruneExpiredLocked(key, entry) {
			return s.createSetEntryLocked(key)
		}
		if entry.Type != TypeSet {
			return nil, nil, ErrWrongType
		}
		set, ok := entry.Value.(map[string]struct{})
		if !ok {
			return nil, nil, ErrInvalidValue
		}
		return set, entry, nil
	}

	return s.createSetEntryLocked(key)
}

func (s *Store) createSetEntryLocked(key string) (map[string]struct{}, *Entry, error) {
	now := time.Now()
	set := make(map[string]struct{})
	entry := &Entry{
		Key:       key,
		Type:      TypeSet,
		Value:     set,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.entries[key] = entry
	return set, entry, nil
}

func (s *Store) getSetLocked(key string) (map[string]struct{}, *Entry, bool, error) {
	entry, ok := s.entries[key]
	if !ok || s.pruneExpiredLocked(key, entry) {
		return nil, nil, false, nil
	}
	if entry.Type != TypeSet {
		return nil, nil, false, ErrWrongType
	}

	set, ok := entry.Value.(map[string]struct{})
	if !ok {
		return nil, nil, false, ErrInvalidValue
	}
	return set, entry, true, nil
}

func (s *Store) getSetRLocked(key string) (map[string]struct{}, *Entry, bool, error) {
	entry, ok := s.entries[key]
	if !ok || s.isExpired(entry) {
		return nil, nil, false, nil
	}
	if entry.Type != TypeSet {
		return nil, nil, false, ErrWrongType
	}

	set, ok := entry.Value.(map[string]struct{})
	if !ok {
		return nil, nil, false, ErrInvalidValue
	}
	return set, entry, true, nil
}
