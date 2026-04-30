package store

import (
	"strconv"
	"time"
)

func (s *Store) IncrString(key string) (int64, error) {
	if key == "" {
		return 0, ErrEmptyKey
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[key]
	if ok && !s.pruneExpiredLocked(key, entry) {
		if entry.Type != TypeString {
			return 0, ErrWrongType
		}
		value, ok := entry.Value.(string)
		if !ok {
			return 0, ErrInvalidValue
		}
		num, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, ErrNotInteger
		}
		num++
		entry.Value = strconv.FormatInt(num, 10)
		entry.UpdatedAt = time.Now()
		return num, nil
	}

	now := time.Now()
	s.entries[key] = &Entry{
		Key:       key,
		Type:      TypeString,
		Value:     "1",
		CreatedAt: now,
		UpdatedAt: now,
	}
	return 1, nil
}

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

	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.entries[key]
	if !ok || s.isExpired(entry) {
		return "", false, nil
	}
	if entry.Type != TypeString {
		return "", false, ErrWrongType
	}

	value, ok := entry.Value.(string)
	if !ok {
		return "", false, ErrInvalidValue
	}

	return value, true, nil
}
