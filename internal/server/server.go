package server

import (
	"fmt"
	"net"
	"sync"

	"gomemkv/internal/core/cmd_handler"
	"gomemkv/internal/pubsub"
	"gomemkv/internal/store"
)

// TCPServer is a RESP-compatible TCP server.
type TCPServer struct {
	listener net.Listener
	handler  *cmd_handler.CommandHandler
	hub      *pubsub.Hub

	mu     sync.Mutex
	conns  map[net.Conn]struct{}
	closed bool
	done   chan struct{}
}

// NewServer creates a new TCPServer bound to addr.
func NewServer(addr string, s *store.Store, h *pubsub.Hub) (*TCPServer, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}

	return &TCPServer{
		listener: listener,
		handler:  cmd_handler.New(s, h),
		hub:      h,
		conns:    make(map[net.Conn]struct{}),
		done:     make(chan struct{}),
	}, nil
}

// Addr returns the listener's network address.
func (t *TCPServer) Addr() net.Addr {
	return t.listener.Addr()
}

// Serve accepts connections and handles them. Blocks until the listener is
// closed or an unrecoverable error occurs.
func (t *TCPServer) Serve() error {
	defer close(t.done)

	for {
		conn, err := t.listener.Accept()
		if err != nil {
			t.mu.Lock()
			closed := t.closed
			t.mu.Unlock()
			if closed {
				return nil
			}
			return fmt.Errorf("accept: %w", err)
		}

		t.mu.Lock()
		t.conns[conn] = struct{}{}
		t.mu.Unlock()

		c := newClient(conn, t.handler, t.hub)
		go func() {
			c.handleLoop()
			t.mu.Lock()
			delete(t.conns, conn)
			t.mu.Unlock()
		}()
	}
}

// GracefulStop stops accepting new connections and waits for existing
// connections to finish.
func (t *TCPServer) GracefulStop() {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()

	t.listener.Close()
	<-t.done
}

// Stop forcibly closes all connections and the listener.
func (t *TCPServer) Stop() {
	t.mu.Lock()
	t.closed = true
	for conn := range t.conns {
		conn.Close()
	}
	t.mu.Unlock()

	t.listener.Close()
}
