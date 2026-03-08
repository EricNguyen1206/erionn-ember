package cache

import (
	"container/list"
	"sync"
	"time"
)

// Entry holds cached prompt/response data.
type Entry struct {
	ID                   string
	PromptHash           uint64
	Tokens               []string // normalized tokens for BM25+Jaccard similarity scoring
	NormalizedPrompt     string
	CompressedPrompt     []byte
	CompressedResponse   []byte
	OriginalPromptSize   int
	OriginalResponseSize int
	CreatedAt            time.Time
	LastAccessed         time.Time
	AccessCount          int
	ExpiresAt            *time.Time
}

type lruItem struct {
	hash  uint64
	entry *Entry
}

// MetadataStore is a thread-safe LRU cache with optional TTL.
type MetadataStore struct {
	mu      sync.Mutex
	maxSize int
	byHash  map[uint64]*list.Element
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
		byHash:  make(map[uint64]*list.Element, maxSize),
		lru:     list.New(),
	}
}

// Set stores an entry in the cache. ttl == 0 means no expiry.
// If the cache is full, the least recently used item is evicted.
func (s *MetadataStore) Set(hash uint64, entry *Entry, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ttl > 0 {
		expiration := time.Now().Add(ttl)
		entry.ExpiresAt = &expiration
	}

	if element, found := s.byHash[hash]; found {
		element.Value.(*lruItem).entry = entry
		s.lru.MoveToFront(element)
		return
	}

	for s.lru.Len() >= s.maxSize {
		s.evictOldest()
	}

	newItem := &lruItem{hash: hash, entry: entry}
	s.byHash[hash] = s.lru.PushFront(newItem)
}

// FindByHash looks up an entry by its prompt hash.
// Returns nil if the entry is missing or has expired.
// On hit, the entry is moved to the front of the LRU list.
func (s *MetadataStore) FindByHash(hash uint64) *Entry {
	s.mu.Lock()
	defer s.mu.Unlock()

	element, found := s.byHash[hash]
	if !found {
		return nil
	}

	item := element.Value.(*lruItem)
	if s.isExpired(item.entry) {
		s.removeElement(element)
		return nil
	}

	item.entry.LastAccessed = time.Now()
	item.entry.AccessCount++
	s.lru.MoveToFront(element)

	return item.entry
}

// ScanAll returns a snapshot of all live (non-expired) entries for BM25+Jaccard search.
// Callers must not modify the returned entries.
func (s *MetadataStore) ScanAll() []*Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Entry, 0, s.lru.Len())
	for el := s.lru.Front(); el != nil; el = el.Next() {
		item := el.Value.(*lruItem)
		if !s.isExpired(item.entry) {
			out = append(out, item.entry)
		}
	}
	return out
}

// Delete removes an entry by hash. Returns true if the entry was found and deleted.
func (s *MetadataStore) Delete(hash uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	element, found := s.byHash[hash]
	if !found {
		return false
	}

	s.removeElement(element)
	return true
}

// Len returns number of stored entries.
func (s *MetadataStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lru.Len()
}

// Clear removes all entries.
func (s *MetadataStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byHash = make(map[uint64]*list.Element, s.maxSize)
	s.lru.Init()
}

// Stats returns aggregate size and compressed byte count.
func (s *MetadataStore) Stats() (totalEntries, totalCompressedBytes int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for el := s.lru.Front(); el != nil; el = el.Next() {
		item := el.Value.(*lruItem)
		totalEntries++
		totalCompressedBytes += len(item.entry.CompressedPrompt) + len(item.entry.CompressedResponse)
	}
	return
}

func (s *MetadataStore) isExpired(e *Entry) bool {
	return e.ExpiresAt != nil && time.Now().After(*e.ExpiresAt)
}

func (s *MetadataStore) evictOldest() {
	if el := s.lru.Back(); el != nil {
		s.removeElement(el)
	}
}

func (s *MetadataStore) removeElement(el *list.Element) {
	item := el.Value.(*lruItem)
	s.lru.Remove(el)
	delete(s.byHash, item.hash)
}
