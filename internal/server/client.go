package server

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"

	"gomemkv/internal/core"
	"gomemkv/internal/core/cmd_handler"
	"gomemkv/internal/pubsub"
)

// client represents a single TCP connection with optional pub/sub state.
type client struct {
	conn    net.Conn
	reader  *core.RESPReader
	handler *cmd_handler.CommandHandler
	hub     *pubsub.Hub

	writeMu sync.Mutex
	sub     *pubsub.Subscriber
	done    chan struct{}
}

func newClient(conn net.Conn, handler *cmd_handler.CommandHandler, hub *pubsub.Hub) *client {
	return &client{
		conn:    conn,
		reader:  core.NewRESPReader(conn),
		handler: handler,
		hub:     hub,
		done:    make(chan struct{}),
	}
}

// handleLoop reads commands from the connection and dispatches them.
func (c *client) handleLoop() {
	defer c.cleanup()

	for {
		cmd, err := c.reader.ReadCommand()
		if err != nil {
			if err != io.EOF {
				slog.Debug("client read error", "remote", c.conn.RemoteAddr(), "err", err)
			}
			return
		}

		upperCmd := strings.ToUpper(cmd.Cmd)

		// Handle subscription commands at connection level
		switch upperCmd {
		case "SUBSCRIBE":
			if err := c.cmdSUBSCRIBE(cmd.Args); err != nil {
				return
			}
			continue
		case "UNSUBSCRIBE":
			if err := c.cmdUNSUBSCRIBE(cmd.Args); err != nil {
				return
			}
			continue
		}

		// If in subscription mode, only allow SUBSCRIBE, UNSUBSCRIBE, PING
		if c.sub != nil {
			if upperCmd == "PING" {
				if err := c.writeResp(c.handler.Execute(cmd)); err != nil {
					return
				}
				continue
			}
			if err := c.writeResp(respErr("ERR only (P)SUBSCRIBE / (P)UNSUBSCRIBE / PING are allowed in this context")); err != nil {
				return
			}
			continue
		}

		// Normal command execution
		res := c.handler.Execute(cmd)
		if err := c.writeResp(res); err != nil {
			return
		}
	}
}

func (c *client) cmdSUBSCRIBE(args []string) error {
	if len(args) == 0 {
		return c.writeResp(respErr("ERR wrong number of arguments for 'SUBSCRIBE' command"))
	}

	// Lazy-init subscriber on first SUBSCRIBE
	if c.sub == nil {
		c.sub = c.hub.NewSubscriber()
		go c.pushMessages()
	}

	for _, channel := range args {
		c.hub.AddChannels(c.sub.ID, []string{channel})
		count := len(c.sub.Channels)
		resp := encodeSubResponse("subscribe", channel, count)
		if err := c.writeResp(resp); err != nil {
			return err
		}
	}
	return nil
}

func (c *client) cmdUNSUBSCRIBE(args []string) error {
	if c.sub == nil {
		// Not subscribed — send unsubscribe with 0 count
		if len(args) == 0 {
			return c.writeResp(encodeSubResponse("unsubscribe", "", 0))
		}
		for _, channel := range args {
			if err := c.writeResp(encodeSubResponse("unsubscribe", channel, 0)); err != nil {
				return err
			}
		}
		return nil
	}

	// If no args, unsubscribe from all channels
	channels := args
	if len(channels) == 0 {
		channels = append([]string(nil), c.sub.Channels...)
	}

	for _, channel := range channels {
		remaining := c.hub.RemoveChannels(c.sub.ID, []string{channel})
		if err := c.writeResp(encodeSubResponse("unsubscribe", channel, remaining)); err != nil {
			return err
		}

		// If no channels remaining, subscriber was auto-removed by hub
		if remaining == 0 {
			close(c.done)
			c.sub = nil
			c.done = make(chan struct{})
			break
		}
	}
	return nil
}

// pushMessages reads from the subscriber's message channel and pushes
// RESP-encoded messages to the TCP connection.
func (c *client) pushMessages() {
	for {
		select {
		case msg, ok := <-c.sub.Messages:
			if !ok {
				return
			}
			resp := encodeMessagePush(msg.Channel, msg.Payload)
			if err := c.writeResp(resp); err != nil {
				return
			}
		case <-c.done:
			return
		}
	}
}

// writeResp writes data to the connection with mutex protection.
func (c *client) writeResp(data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err := c.conn.Write(data)
	if err != nil {
		slog.Debug("client write error", "remote", c.conn.RemoteAddr(), "err", err)
	}
	return err
}

func (c *client) cleanup() {
	if c.sub != nil {
		close(c.done)
		c.hub.Remove(c.sub.ID)
		c.sub = nil
	}
	c.conn.Close()
}

// --- RESP encoding helpers ---

// encodeSubResponse encodes a subscribe/unsubscribe response:
//
//	*3\r\n$<len>\r\n<type>\r\n$<len>\r\n<channel>\r\n:<count>\r\n
func encodeSubResponse(msgType, channel string, count int) []byte {
	return []byte(fmt.Sprintf("*3\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n:%d\r\n",
		len(msgType), msgType,
		len(channel), channel,
		count,
	))
}

// encodeMessagePush encodes a pub/sub message push:
//
//	*3\r\n$7\r\nmessage\r\n$<len>\r\n<channel>\r\n$<len>\r\n<payload>\r\n
func encodeMessagePush(channel string, payload []byte) []byte {
	return []byte(fmt.Sprintf("*3\r\n$7\r\nmessage\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n",
		len(channel), channel,
		len(payload), payload,
	))
}

func respErr(msg string) []byte {
	return []byte(fmt.Sprintf("-%s\r\n", msg))
}
