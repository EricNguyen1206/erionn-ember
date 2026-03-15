package pubsub

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type Message struct {
	Channel     string
	Payload     []byte
	PublishedAt time.Time
}

type Subscriber struct {
	ID       string
	Channels []string
	Messages chan Message
}

type Stats struct {
	Channels    int64
	Subscribers int64
}

type Hub struct {
	mu     sync.Mutex
	nextID atomic.Int64
	buffer int
	routes map[string]map[string]*Subscriber
	byID   map[string]*Subscriber
}

func New(buffer int) *Hub {
	if buffer <= 0 {
		buffer = 1
	}
	return &Hub{
		buffer: buffer,
		routes: make(map[string]map[string]*Subscriber),
		byID:   make(map[string]*Subscriber),
	}
}

func (h *Hub) Subscribe(channels []string) *Subscriber {
	h.mu.Lock()
	defer h.mu.Unlock()

	id := strconv.FormatInt(h.nextID.Add(1), 10)
	uniqueChannels := uniqueStrings(channels)
	sub := &Subscriber{
		ID:       id,
		Channels: uniqueChannels,
		Messages: make(chan Message, h.buffer),
	}
	h.byID[id] = sub
	for _, channel := range uniqueChannels {
		if h.routes[channel] == nil {
			h.routes[channel] = make(map[string]*Subscriber)
		}
		h.routes[channel][id] = sub
	}
	return sub
}

func (h *Hub) Remove(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.removeLocked(id)
}

func (h *Hub) Publish(channel string, payload []byte) int {
	h.mu.Lock()
	defer h.mu.Unlock()

	subscribers := h.routes[channel]
	if len(subscribers) == 0 {
		return 0
	}

	delivered := 0
	toRemove := make([]string, 0)
	publishedAt := time.Now()
	for id, sub := range subscribers {
		msg := Message{
			Channel:     channel,
			Payload:     append([]byte(nil), payload...),
			PublishedAt: publishedAt,
		}
		select {
		case sub.Messages <- msg:
			delivered++
		default:
			toRemove = append(toRemove, id)
		}
	}

	for _, id := range toRemove {
		h.removeLocked(id)
	}
	return delivered
}

func (h *Hub) Stats() Stats {
	h.mu.Lock()
	defer h.mu.Unlock()
	return Stats{Channels: int64(len(h.routes)), Subscribers: int64(len(h.byID))}
}

func (h *Hub) removeLocked(id string) {
	sub, ok := h.byID[id]
	if !ok {
		return
	}
	delete(h.byID, id)
	for _, channel := range sub.Channels {
		subscribers := h.routes[channel]
		delete(subscribers, id)
		if len(subscribers) == 0 {
			delete(h.routes, channel)
		}
	}
	close(sub.Messages)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
