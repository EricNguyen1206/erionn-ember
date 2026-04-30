package server

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"gomemkv/internal/handler"
	"gomemkv/internal/pubsub"
	"gomemkv/pkg/resp"
)

type client struct {
	conn        net.Conn
	reader      *resp.Reader
	handler     *handler.CommandHandler
	hub         *pubsub.Hub
	idleTimeout time.Duration

	writeMu  sync.Mutex
	sub      *pubsub.Subscriber
	done     chan struct{}

	txActive bool
	txQueue  []*resp.Command
}

func newClient(conn net.Conn, handler *handler.CommandHandler, hub *pubsub.Hub, idleTimeout time.Duration) *client {
	return &client{
		conn:        conn,
		reader:      resp.NewReader(conn),
		handler:     handler,
		hub:         hub,
		idleTimeout: idleTimeout,
		done:        make(chan struct{}),
	}
}

func (c *client) handleLoop() {
	defer c.cleanup()

	for {
		if c.idleTimeout > 0 {
			_ = c.conn.SetReadDeadline(time.Now().Add(c.idleTimeout))
		}

		cmd, err := c.reader.ReadCommand()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				slog.Debug("client idle timeout", "remote", c.conn.RemoteAddr())
			} else if err != io.EOF {
				slog.Debug("client read error", "remote", c.conn.RemoteAddr(), "err", err)
			}
			return
		}

		upperCmd := strings.ToUpper(cmd.Cmd)

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
		case "QUIT":
			_ = c.writeResp(resp.Ok)
			return
		case "MULTI":
			if c.txActive {
				if err := c.writeResp(resp.EncodeError("ERR MULTI calls can not be nested")); err != nil {
					return
				}
				continue
			}
			c.txActive = true
			c.txQueue = c.txQueue[:0]
			if err := c.writeResp(resp.Ok); err != nil {
				return
			}
			continue
		case "EXEC":
			if !c.txActive {
				if err := c.writeResp(resp.EncodeError("ERR EXEC without MULTI")); err != nil {
					return
				}
				continue
			}
			c.txActive = false
			results := make([][]byte, len(c.txQueue))
			for i, queued := range c.txQueue {
				results[i] = c.handler.Execute(queued)
			}
			c.txQueue = c.txQueue[:0]
			if err := c.writeResp(encodeTxResults(results)); err != nil {
				return
			}
			continue
		case "DISCARD":
			if !c.txActive {
				if err := c.writeResp(resp.EncodeError("ERR DISCARD without MULTI")); err != nil {
					return
				}
				continue
			}
			c.txActive = false
			c.txQueue = c.txQueue[:0]
			if err := c.writeResp(resp.Ok); err != nil {
				return
			}
			continue
		}

		// Queue command if inside MULTI
		if c.txActive {
			c.txQueue = append(c.txQueue, cmd)
			if err := c.writeResp([]byte("+QUEUED\r\n")); err != nil {
				return
			}
			continue
		}

		if c.sub != nil {
			if upperCmd == "PING" {
				if err := c.writeResp(c.handler.Execute(cmd)); err != nil {
					return
				}
				continue
			}
			if err := c.writeResp(resp.EncodeError("ERR only (P)SUBSCRIBE / (P)UNSUBSCRIBE / PING are allowed in this context")); err != nil {
				return
			}
			continue
		}

		res := c.handler.Execute(cmd)
		if err := c.writeResp(res); err != nil {
			return
		}
	}
}

func (c *client) cmdSUBSCRIBE(args []string) error {
	if len(args) == 0 {
		return c.writeResp(resp.EncodeError("ERR wrong number of arguments for 'SUBSCRIBE' command"))
	}

	if c.sub == nil {
		c.sub = c.hub.NewSubscriber()
		go c.pushMessages()
	}

	for _, channel := range args {
		c.hub.AddChannels(c.sub.ID, []string{channel})
		count := len(c.sub.Channels)
		data := encodeSubResponse("subscribe", channel, count)
		if err := c.writeResp(data); err != nil {
			return err
		}
	}
	return nil
}

func (c *client) cmdUNSUBSCRIBE(args []string) error {
	if c.sub == nil {
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

	channels := args
	if len(channels) == 0 {
		channels = append([]string(nil), c.sub.Channels...)
	}

	for _, channel := range channels {
		remaining := c.hub.RemoveChannels(c.sub.ID, []string{channel})
		if err := c.writeResp(encodeSubResponse("unsubscribe", channel, remaining)); err != nil {
			return err
		}

		if remaining == 0 {
			close(c.done)
			c.sub = nil
			c.done = make(chan struct{})
			break
		}
	}
	return nil
}

func (c *client) pushMessages() {
	for {
		select {
		case msg, ok := <-c.sub.Messages:
			if !ok {
				return
			}
			data := encodeMessagePush(msg.Channel, msg.Payload)
			if err := c.writeResp(data); err != nil {
				return
			}
		case <-c.done:
			return
		}
	}
}

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

func encodeSubResponse(msgType, channel string, count int) []byte {
	return []byte(fmt.Sprintf("*3\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n:%d\r\n",
		len(msgType), msgType,
		len(channel), channel,
		count,
	))
}

func encodeMessagePush(channel string, payload []byte) []byte {
	return []byte(fmt.Sprintf("*3\r\n$7\r\nmessage\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n",
		len(channel), channel,
		len(payload), payload,
	))
}

func encodeTxResults(results [][]byte) []byte {
	header := fmt.Sprintf("*%d\r\n", len(results))
	total := len(header)
	for _, r := range results {
		total += len(r)
	}
	buf := make([]byte, 0, total)
	buf = append(buf, header...)
	for _, r := range results {
		buf = append(buf, r...)
	}
	return buf
}
