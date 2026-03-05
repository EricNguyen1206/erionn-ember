package cache

import (
	"container/list"
	"sync"
	"time"
)

// Entry holds cached prompt/response data.
type Entry struct {
	ID                   string
	VectorID             int
	PromptHash           uint64
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
	byVecID map[int]*list.Element
	lru     *list.List
}

func NewMetadataStore(maxSize int) *MetadataStore {
	if maxSize <= 0 {
		maxSize = 100000
	}
	return &MetadataStore{
		maxSize: maxSize,
		byHash:  make(map[uint64]*list.Element, maxSize),
		byVecID: make(map[int]*list.Element, maxSize),
		lru:     list.New(),
	}
}

// Set stores an entry. ttl == 0 means no expiry.
func (s *MetadataStore) Set(hash uint64, e *Entry, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ttl > 0 {
		exp := time.Now().Add(ttl)
		e.ExpiresAt = &exp
	}
	if el, ok := s.byHash[hash]; ok {
		item := el.Value.(*lruItem)
		delete(s.byVecID, item.entry.VectorID)
		item.entry = e
		s.byVecID[e.VectorID] = el
		s.lru.MoveToFront(el)
		return
	}
	for s.lru.Len() >= s.maxSize {
		s.evictOldest()
	}
	el := s.lru.PushFront(&lruItem{hash: hash, entry: e})
	s.byHash[hash] = el
	s.byVecID[e.VectorID] = el
}

// FindByHash looks up by prompt hash. Returns nil if missing or expired.
func (s *MetadataStore) FindByHash(hash uint64) *Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	el, ok := s.byHash[hash]
	if !ok {
		return nil
	}
	item := el.Value.(*lruItem)
	if s.isExpired(item.entry) {
		s.removeElement(el)
		return nil
	}
	item.entry.LastAccessed = time.Now()
	item.entry.AccessCount++
	s.lru.MoveToFront(el)
	return item.entry
}

// FindByVectorID looks up by vector index ID.
func (s *MetadataStore) FindByVectorID(vectorID int) *Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	el, ok := s.byVecID[vectorID]
	if !ok {
		return nil
	}
	item := el.Value.(*lruItem)
	if s.isExpired(item.entry) {
		s.removeElement(el)
		return nil
	}
	item.entry.LastAccessed = time.Now()
	item.entry.AccessCount++
	s.lru.MoveToFront(el)
	return item.entry
}

// Delete removes by hash. Returns true if found.
func (s *MetadataStore) Delete(hash uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	el, ok := s.byHash[hash]
	if !ok {
		return false
	}
	s.removeElement(el)
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
	s.byVecID = make(map[int]*list.Element, s.maxSize)
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
	delete(s.byVecID, item.entry.VectorID)
}
