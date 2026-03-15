package store

import "time"

func (s *Store) HSet(key string, fields map[string]string) (int, error) {
	if key == "" {
		return 0, ErrEmptyKey
	}
	if len(fields) == 0 {
		return 0, ErrEmptyFieldSet
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	hash, entry, err := s.getOrCreateHashLocked(key)
	if err != nil {
		return 0, err
	}

	added := 0
	for field, value := range fields {
		if _, exists := hash[field]; !exists {
			added++
		}
		hash[field] = value
	}
	entry.UpdatedAt = time.Now()
	return added, nil
}

func (s *Store) HGet(key, field string) (string, bool, error) {
	if key == "" {
		return "", false, ErrEmptyKey
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	hash, entry, found, err := s.getHashLocked(key)
	if err != nil || !found {
		return "", false, err
	}

	value, ok := hash[field]
	if ok {
		entry.UpdatedAt = time.Now()
	}
	return value, ok, nil
}

func (s *Store) HGetAll(key string) (map[string]string, bool, error) {
	if key == "" {
		return nil, false, ErrEmptyKey
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	hash, entry, found, err := s.getHashLocked(key)
	if err != nil || !found {
		return nil, false, err
	}

	copyFields := make(map[string]string, len(hash))
	for field, value := range hash {
		copyFields[field] = value
	}
	entry.UpdatedAt = time.Now()
	return copyFields, true, nil
}

func (s *Store) HDel(key string, fields []string) (int, error) {
	if key == "" {
		return 0, ErrEmptyKey
	}
	if len(fields) == 0 {
		return 0, ErrEmptyFieldSet
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	hash, entry, found, err := s.getHashLocked(key)
	if err != nil || !found {
		return 0, err
	}

	removed := 0
	for _, field := range fields {
		if _, exists := hash[field]; exists {
			delete(hash, field)
			removed++
		}
	}
	if len(hash) == 0 {
		delete(s.entries, key)
		return removed, nil
	}
	if removed > 0 {
		entry.UpdatedAt = time.Now()
	}
	return removed, nil
}

func (s *Store) getOrCreateHashLocked(key string) (map[string]string, *Entry, error) {
	if entry, ok := s.entries[key]; ok {
		if s.pruneExpiredLocked(key, entry) {
			return s.createHashEntryLocked(key)
		}
		if entry.Type != TypeHash {
			return nil, nil, ErrWrongType
		}
		hash, ok := entry.Value.(map[string]string)
		if !ok {
			return nil, nil, ErrInvalidValue
		}
		return hash, entry, nil
	}

	return s.createHashEntryLocked(key)
}

func (s *Store) createHashEntryLocked(key string) (map[string]string, *Entry, error) {
	now := time.Now()
	hash := make(map[string]string)
	entry := &Entry{
		Key:       key,
		Type:      TypeHash,
		Value:     hash,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.entries[key] = entry
	return hash, entry, nil
}

func (s *Store) getHashLocked(key string) (map[string]string, *Entry, bool, error) {
	entry, ok := s.entries[key]
	if !ok || s.pruneExpiredLocked(key, entry) {
		return nil, nil, false, nil
	}
	if entry.Type != TypeHash {
		return nil, nil, false, ErrWrongType
	}

	hash, ok := entry.Value.(map[string]string)
	if !ok {
		return nil, nil, false, ErrInvalidValue
	}
	return hash, entry, true, nil
}
