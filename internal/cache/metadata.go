package cache

import (
	"container/list"
	"sync"
	"time"
)

// Entry holds cached prompt/response data.
type Entry struct {
	ID                   string
	NamespaceKey         string
	PromptHash           uint64
	Tokens               []string // normalized tokens for BM25+Jaccard similarity scoring
	Vector               []float32
	NormalizedPrompt     string
	CompressedPrompt     []byte
	CompressedResponse   []byte
	OriginalPromptSize   int
	OriginalResponseSize int
	CreatedAt            time.Time
	LastAccessed         time.Time
	AccessCount          int
	ExpiresAt            time.Time
}

type exactKey struct {
	namespaceKey string
	promptHash   uint64
}

type lruItem struct {
	key   exactKey
	entry *Entry
}

// MetadataStore is a thread-safe LRU cache with optional TTL.
type MetadataStore struct {
	mu      sync.Mutex
	maxSize int
	byHash  map[exactKey]*list.Element
	lru     *list.List
}

// NewMetadataStore creates a new MetadataStore with the given maxSize.
// If maxSize is <= 0, a default of 100,000 is used.
func NewMetadataStore(maxSize int) *MetadataStore {
	if maxSize <= 0 {
		maxSize = 100000
	}
	return &MetadataStore{
		maxSize: maxSize,
		byHash:  make(map[exactKey]*list.Element, maxSize),
		lru:     list.New(),
	}
}

// Set stores an entry in the cache. ttl == 0 means no expiry.
// If the cache is full, the least recently used item is evicted.
// It returns any replaced or evicted entry so callers can keep secondary indexes in sync.
func (s *MetadataStore) Set(hash uint64, entry *Entry, ttl time.Duration) (replaced *Entry, evicted *Entry) {
	return s.SetExact("", hash, entry, ttl)
}

// SetExact stores an entry in the cache under a namespace-aware exact lookup key.
func (s *MetadataStore) SetExact(namespaceKey string, hash uint64, entry *Entry, ttl time.Duration) (replaced *Entry, evicted *Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	entry.NamespaceKey = namespaceKey
	entry.PromptHash = hash
	if ttl > 0 {
		entry.ExpiresAt = now.Add(ttl)
	} else {
		entry.ExpiresAt = time.Time{}
	}

	key := exactLookupKey(namespaceKey, hash)
	if element, found := s.byHash[key]; found {
		item := element.Value.(*lruItem)
		replaced = item.entry
		item.key = key
		item.entry = entry
		s.lru.MoveToFront(element)
		return replaced, nil
	}

	if s.lru.Len() >= s.maxSize {
		evicted = s.evictOneForInsertLocked(now)
	}

	newItem := &lruItem{key: key, entry: entry}
	s.byHash[key] = s.lru.PushFront(newItem)
	return nil, evicted
}

// FindByHash looks up an entry by its prompt hash.
// Returns nil if the entry is missing or has expired.
// On hit, the entry is moved to the front of the LRU list.
func (s *MetadataStore) FindByHash(hash uint64) *Entry {
	entry, _ := s.FindByHashWithExpired(hash)
	return entry
}

// FindByHashWithExpired looks up an entry and also returns any expired entry pruned during lookup.
func (s *MetadataStore) FindByHashWithExpired(hash uint64) (entry *Entry, expired *Entry) {
	return s.FindExactByHashWithExpired("", hash)
}

// FindExactByHash looks up an entry by namespace-aware prompt hash.
func (s *MetadataStore) FindExactByHash(namespaceKey string, hash uint64) *Entry {
	entry, _ := s.FindExactByHashWithExpired(namespaceKey, hash)
	return entry
}

// FindExactByHashWithExpired looks up a namespaced entry and returns any expired entry pruned during lookup.
func (s *MetadataStore) FindExactByHashWithExpired(namespaceKey string, hash uint64) (entry *Entry, expired *Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.findByHashLocked(exactLookupKey(namespaceKey, hash), time.Now())
}

// ScanAll returns a snapshot of all live (non-expired) entries for BM25+Jaccard search.
// Callers must not modify the returned entries.
func (s *MetadataStore) ScanAll() []*Entry {
	entries, _ := s.ScanAllLive()
	return entries
}

// ScanAllLive returns all live entries plus any expired entries that were removed while scanning.
func (s *MetadataStore) ScanAllLive() (entries []*Entry, expired []*Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	entries = make([]*Entry, 0, s.lru.Len())
	for el := s.lru.Front(); el != nil; {
		next := el.Next()
		item := el.Value.(*lruItem)
		if s.isExpiredAt(item.entry, now) {
			expired = append(expired, s.removeElementLocked(el))
		} else {
			entries = append(entries, item.entry)
		}
		el = next
	}

	return entries, expired
}

// Delete removes an entry by hash. Returns true if the entry was found and deleted.
func (s *MetadataStore) Delete(hash uint64) bool {
	_, deleted := s.DeleteEntry(hash)
	return deleted
}

// DeleteEntry removes an entry by hash and returns the removed entry when present.
func (s *MetadataStore) DeleteEntry(hash uint64) (*Entry, bool) {
	return s.DeleteExactEntry("", hash)
}

// DeleteExact removes a namespaced entry by hash.
func (s *MetadataStore) DeleteExact(namespaceKey string, hash uint64) bool {
	_, deleted := s.DeleteExactEntry(namespaceKey, hash)
	return deleted
}

// DeleteExactEntry removes a namespaced entry by hash and returns the removed entry when present.
func (s *MetadataStore) DeleteExactEntry(namespaceKey string, hash uint64) (*Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	element, found := s.byHash[exactLookupKey(namespaceKey, hash)]
	if !found {
		return nil, false
	}

	return s.removeElementLocked(element), true
}

// Len returns number of stored live entries.
func (s *MetadataStore) Len() int {
	entries, _ := s.ScanAllLive()
	return len(entries)
}

// Clear removes all entries.
func (s *MetadataStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byHash = make(map[exactKey]*list.Element, s.maxSize)
	s.lru.Init()
}

// Stats returns aggregate size and compressed byte count for live entries.
func (s *MetadataStore) Stats() (totalEntries, totalCompressedBytes int) {
	totalEntries, totalCompressedBytes, _ = s.StatsLive()
	return totalEntries, totalCompressedBytes
}

// StatsLive returns aggregate size and compressed byte count for live entries.
// It also returns any expired entries removed while computing stats.
func (s *MetadataStore) StatsLive() (totalEntries, totalCompressedBytes int, expired []*Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for el := s.lru.Front(); el != nil; {
		next := el.Next()
		item := el.Value.(*lruItem)
		if s.isExpiredAt(item.entry, now) {
			expired = append(expired, s.removeElementLocked(el))
		} else {
			totalEntries++
			totalCompressedBytes += len(item.entry.CompressedPrompt) + len(item.entry.CompressedResponse)
		}
		el = next
	}

	return totalEntries, totalCompressedBytes, expired
}

func (s *MetadataStore) findByHashLocked(key exactKey, now time.Time) (entry *Entry, expired *Entry) {
	element, found := s.byHash[key]
	if !found {
		return nil, nil
	}

	item := element.Value.(*lruItem)
	if s.isExpiredAt(item.entry, now) {
		return nil, s.removeElementLocked(element)
	}

	item.entry.LastAccessed = now
	item.entry.AccessCount++
	s.lru.MoveToFront(element)

	return item.entry, nil
}

func (s *MetadataStore) isExpiredAt(e *Entry, now time.Time) bool {
	return !e.ExpiresAt.IsZero() && now.After(e.ExpiresAt)
}

func (s *MetadataStore) evictOneForInsertLocked(now time.Time) *Entry {
	if el := s.lru.Back(); el != nil {
		item := el.Value.(*lruItem)
		if s.isExpiredAt(item.entry, now) {
			return s.removeElementLocked(el)
		}
	}
	return s.evictOldestLocked()
}

func (s *MetadataStore) evictOldestLocked() *Entry {
	if el := s.lru.Back(); el != nil {
		return s.removeElementLocked(el)
	}
	return nil
}

func (s *MetadataStore) removeElementLocked(el *list.Element) *Entry {
	item := el.Value.(*lruItem)
	s.lru.Remove(el)
	delete(s.byHash, item.key)
	return item.entry
}

func exactLookupKey(namespaceKey string, hash uint64) exactKey {
	return exactKey{namespaceKey: namespaceKey, promptHash: hash}
}
