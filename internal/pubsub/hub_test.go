package pubsub

import (
	"testing"
	"time"
)

func TestHubPublishDeliversToActiveSubscribers(t *testing.T) {
	hub := New(1)
	sub := hub.Subscribe([]string{"news"})
	defer hub.Remove(sub.ID)

	if delivered := hub.Publish("news", []byte("hello")); delivered != 1 {
		t.Fatalf("got %d deliveries, want 1", delivered)
	}

	select {
	case msg := <-sub.Messages:
		if msg.Channel != "news" || string(msg.Payload) != "hello" {
			t.Fatalf("unexpected message: %+v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for published message")
	}
}

func TestHubDropsSlowSubscriberWhenBufferIsFull(t *testing.T) {
	hub := New(1)
	sub := hub.Subscribe([]string{"news"})

	if delivered := hub.Publish("news", []byte("first")); delivered != 1 {
		t.Fatalf("got %d deliveries, want 1", delivered)
	}
	if delivered := hub.Publish("news", []byte("second")); delivered != 0 {
		t.Fatalf("got %d deliveries on second publish, want 0", delivered)
	}

	stats := hub.Stats()
	if stats.Subscribers != 0 || stats.Channels != 0 {
		t.Fatalf("expected slow subscriber cleanup, got stats=%+v", stats)
	}

	if delivered := hub.Publish("news", []byte("third")); delivered != 0 {
		t.Fatalf("got %d deliveries after cleanup, want 0", delivered)
	}

	select {
	case msg, ok := <-sub.Messages:
		if !ok || string(msg.Payload) != "first" {
			t.Fatalf("unexpected buffered message state: msg=%+v ok=%v", msg, ok)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for buffered message")
	}

	select {
	case _, ok := <-sub.Messages:
		if ok {
			t.Fatal("expected subscriber channel to be closed after buffered message drains")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for closed subscriber channel")
	}
}

func TestHubRemoveUnsubscribesSubscriber(t *testing.T) {
	hub := New(2)
	sub := hub.Subscribe([]string{"news", "ops"})

	hub.Remove(sub.ID)

	stats := hub.Stats()
	if stats.Subscribers != 0 || stats.Channels != 0 {
		t.Fatalf("expected remove to cleanup all routes, got stats=%+v", stats)
	}
	if delivered := hub.Publish("news", []byte("hello")); delivered != 0 {
		t.Fatalf("got %d deliveries, want 0", delivered)
	}
}

func TestHubCopiesPayloadPerSubscriber(t *testing.T) {
	hub := New(1)
	first := hub.Subscribe([]string{"news"})
	second := hub.Subscribe([]string{"news"})
	defer hub.Remove(first.ID)
	defer hub.Remove(second.ID)

	if delivered := hub.Publish("news", []byte("hello")); delivered != 2 {
		t.Fatalf("got %d deliveries, want 2", delivered)
	}

	msg1 := <-first.Messages
	msg2 := <-second.Messages
	msg1.Payload[0] = 'j'

	if string(msg2.Payload) != "hello" {
		t.Fatalf("expected independent payload copy, got %q", string(msg2.Payload))
	}
}
