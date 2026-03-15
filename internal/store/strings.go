package store

import "time"

func (s *Store) SetString(key, value string, ttl time.Duration) error {
	if key == "" {
		return ErrEmptyKey
	}

	now := time.Now()
	entry := &Entry{
		Key:       key,
		Type:      TypeString,
		Value:     value,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if ttl > 0 {
		entry.ExpiresAt = now.Add(ttl)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.entries[key]; ok && !s.pruneExpiredLocked(key, existing) && !existing.CreatedAt.IsZero() {
		entry.CreatedAt = existing.CreatedAt
	}
	s.entries[key] = entry
	return nil
}

func (s *Store) GetString(key string) (string, bool, error) {
	if key == "" {
		return "", false, ErrEmptyKey
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[key]
	if !ok || s.pruneExpiredLocked(key, entry) {
		return "", false, nil
	}
	if entry.Type != TypeString {
		return "", false, ErrWrongType
	}

	value, ok := entry.Value.(string)
	if !ok {
		return "", false, ErrInvalidValue
	}

	entry.UpdatedAt = time.Now()
	return value, true, nil
}
