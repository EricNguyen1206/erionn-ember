package store

import "time"

func (s *Store) LPush(key string, values []string) (int, error) {
	return s.pushList(key, values, true)
}

func (s *Store) RPush(key string, values []string) (int, error) {
	return s.pushList(key, values, false)
}

func (s *Store) LPop(key string) (string, bool, error) {
	return s.popList(key, true)
}

func (s *Store) RPop(key string) (string, bool, error) {
	return s.popList(key, false)
}

func (s *Store) LRange(key string, start, stop int64) ([]string, bool, error) {
	if key == "" {
		return nil, false, ErrEmptyKey
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	list, _, found, err := s.getListRLocked(key)
	if err != nil || !found {
		return nil, false, err
	}

	from, to, ok := normalizeRange(len(list), start, stop)
	if !ok {
		return []string{}, true, nil
	}

	out := append([]string(nil), list[from:to+1]...)
	return out, true, nil
}

func (s *Store) pushList(key string, values []string, left bool) (int, error) {
	if key == "" {
		return 0, ErrEmptyKey
	}
	if len(values) == 0 {
		return 0, ErrEmptyValues
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	list, entry, err := s.getOrCreateListLocked(key)
	if err != nil {
		return 0, err
	}

	if left {
		pushed := make([]string, 0, len(values)+len(list))
		for i := len(values) - 1; i >= 0; i-- {
			pushed = append(pushed, values[i])
		}
		pushed = append(pushed, list...)
		entry.Value = pushed
		entry.UpdatedAt = time.Now()
		return len(pushed), nil
	}

	list = append(list, values...)
	entry.Value = list
	entry.UpdatedAt = time.Now()
	return len(list), nil
}

func (s *Store) popList(key string, left bool) (string, bool, error) {
	if key == "" {
		return "", false, ErrEmptyKey
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	list, entry, found, err := s.getListLocked(key)
	if err != nil || !found {
		return "", false, err
	}
	if len(list) == 0 {
		delete(s.entries, key)
		return "", false, nil
	}

	var item string
	if left {
		item = list[0]
		list = list[1:]
	} else {
		item = list[len(list)-1]
		list = list[:len(list)-1]
	}

	if len(list) == 0 {
		delete(s.entries, key)
		return item, true, nil
	}

	entry.Value = list
	entry.UpdatedAt = time.Now()
	return item, true, nil
}

func (s *Store) getOrCreateListLocked(key string) ([]string, *Entry, error) {
	if entry, ok := s.entries[key]; ok {
		if s.pruneExpiredLocked(key, entry) {
			return s.createListEntryLocked(key)
		}
		if entry.Type != TypeList {
			return nil, nil, ErrWrongType
		}
		list, ok := entry.Value.([]string)
		if !ok {
			return nil, nil, ErrInvalidValue
		}
		return list, entry, nil
	}

	return s.createListEntryLocked(key)
}

func (s *Store) createListEntryLocked(key string) ([]string, *Entry, error) {
	now := time.Now()
	list := make([]string, 0)
	entry := &Entry{
		Key:       key,
		Type:      TypeList,
		Value:     list,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.entries[key] = entry
	return list, entry, nil
}

func (s *Store) getListLocked(key string) ([]string, *Entry, bool, error) {
	entry, ok := s.entries[key]
	if !ok || s.pruneExpiredLocked(key, entry) {
		return nil, nil, false, nil
	}
	if entry.Type != TypeList {
		return nil, nil, false, ErrWrongType
	}

	list, ok := entry.Value.([]string)
	if !ok {
		return nil, nil, false, ErrInvalidValue
	}
	return list, entry, true, nil
}

func (s *Store) getListRLocked(key string) ([]string, *Entry, bool, error) {
	entry, ok := s.entries[key]
	if !ok || s.isExpired(entry) {
		return nil, nil, false, nil
	}
	if entry.Type != TypeList {
		return nil, nil, false, ErrWrongType
	}

	list, ok := entry.Value.([]string)
	if !ok {
		return nil, nil, false, ErrInvalidValue
	}
	return list, entry, true, nil
}

func normalizeRange(length int, start, stop int64) (int, int, bool) {
	if length == 0 {
		return 0, 0, false
	}

	from := normalizeIndex(length, start)
	to := normalizeIndex(length, stop)
	if from < 0 {
		from = 0
	}
	if to >= length {
		to = length - 1
	}
	if from >= length || to < 0 || from > to {
		return 0, 0, false
	}

	return from, to, true
}

func normalizeIndex(length int, index int64) int {
	if index >= 0 {
		return int(index)
	}
	return length + int(index)
}
