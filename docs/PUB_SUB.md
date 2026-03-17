# Pub/Sub Guide (Publish-Subscribe)

Ember is **NOT JUST** an in-memory Key-Value store for computing data; it also acts as a **high-speed Message Broker** for Real-Time execution (Pub/Sub). 

The Pub/Sub architecture in Ember utilizes a Fan-out mechanism via `internal/pubsub` and relies on the long-lived TCP connections of gRPC Streaming.

## How it Works

1. **Client A** continuously listens to (Subscribes) one or more "Channels". This `TCP gRPC` connection is kept alive to receive data silently when new events occur (Push mechanism).
2. **Channels** do not need to be created in advance / They automatically disappear depending on the number of Subscribers.
3. **Client B** sends a message (Publishes) directly to that specific Channel. 
4. The internal Hub uses a Mutex to automatically Fan-out (distribute) copies of the payload to all 24/7 active Subscribers.

---

## 1. Subscribing (Listening to a Channel)

Unlike regular one-time gRPC calls, Subscribe is **Bi-directional or Server-Streaming**, requiring you to loop indefinitely (For Loop) using a lightweight Goroutine. It uses channels named `system_events` and `chat_room`.

```go
// A Background context is required (no Timeout) because it hangs long-term.
stream, err := client.Subscribe(context.Background(), &pb.SubscribeRequest{
	Channels: []string{"system_events", "notifications_user_123"},
})
if err != nil {
	log.Fatalf("Subscribe error: %v", err)
}

// Start listening for Server Stream Messages
for {
	msg, err := stream.Recv()
	if err != nil {
		// Catches network drops / Server shutdown.
		log.Printf("Disconnected: %v", err)
		break
	}
	
	// Print out when a message arrives:
	fmt.Printf("[%s] %s\n", msg.Channel, msg.Payload)
	// Output: "[system_events] System alert: OS update!"
}
```

*Warning: Ember is designed so that if processing code (like the Print above on the Client) bottlenecks (Buffer Too Full - Slow Subscribers), the Ember Hub on the Server will actively throttle and abruptly drop this Subscriber to prevent Server RAM exhaustion. You must process messages as fast as possible!*

## 2. Publishing (Broadcasting Messages)

Whenever other Microservices in your cluster experience an event change (User orders an item, Update data), they just need to briefly call the `Publish` API once. If no one is subscribing at that moment, the message falls straight into the Void (Permanently deleted, not Archived to Disk).

```go
// Fire Message to the Channel
res, err := client.Publish(ctx, &pb.PublishRequest{
	Channel: "notifications_user_123",
	Payload: `{"type": "new_order", "id": 501}`,
})
if err != nil {
	log.Fatal(err)
}

// Easily find out how many actual Clients received the Push Notifications
fmt.Printf("%d machines received this message in realtime.\n", res.Delivered)
```

## Advantages of Using Ember for Pub/Sub
- Avoids the need to configure and install bulky, heavy-duty Brokers like RabbitMQ or ActiveMQ if your System is lightweight.
- Data streams directly through HTTP/2 Multiplexing of gRPC, reducing Ping Delay down to micro-seconds. 
- Does not cause Blocking for other Clients at the Store Core Layer. The Hub is an entity completely independent from `store.Store`.
