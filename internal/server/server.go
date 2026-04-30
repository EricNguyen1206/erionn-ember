package server

import (
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"gomemkv/internal/handler"
	"gomemkv/internal/pubsub"
	"gomemkv/internal/store"
)

type ServerConfig struct {
	MaxConns    int
	IdleTimeout time.Duration
}

type TCPServer struct {
	listener    net.Listener
	handler     *handler.CommandHandler
	hub         *pubsub.Hub
	maxConns    int
	idleTimeout time.Duration

	mu     sync.Mutex
	conns  map[net.Conn]struct{}
	closed bool
	done   chan struct{}
}

func NewServer(addr string, s *store.Store, h *pubsub.Hub, cfg ServerConfig) (*TCPServer, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}

	return &TCPServer{
		listener:    listener,
		handler:     handler.New(s, h),
		hub:         h,
		maxConns:    cfg.MaxConns,
		idleTimeout: cfg.IdleTimeout,
		conns:       make(map[net.Conn]struct{}),
		done:        make(chan struct{}),
	}, nil
}

func (t *TCPServer) Addr() net.Addr {
	return t.listener.Addr()
}

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
		if t.maxConns > 0 && len(t.conns) >= t.maxConns {
			t.mu.Unlock()
			slog.Warn("connection limit reached, rejecting", "remote", conn.RemoteAddr(), "max", t.maxConns)
			conn.Close()
			continue
		}
		t.conns[conn] = struct{}{}
		t.mu.Unlock()

		c := newClient(conn, t.handler, t.hub, t.idleTimeout)
		go func() {
			c.handleLoop()
			t.mu.Lock()
			delete(t.conns, conn)
			t.mu.Unlock()
		}()
	}
}

func (t *TCPServer) GracefulStop() {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()

	t.listener.Close()
	<-t.done
}

func (t *TCPServer) Stop() {
	t.mu.Lock()
	t.closed = true
	for conn := range t.conns {
		conn.Close()
	}
	t.mu.Unlock()

	t.listener.Close()
}
