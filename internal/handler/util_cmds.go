package handler

import (
	"fmt"

	"gomemkv/pkg/resp"
)

func (c *CommandHandler) cmdPING(args []string) []byte {
	if len(args) > 1 {
		return resp.EncodeError("ERR wrong number of arguments for 'PING' command")
	}
	if len(args) == 0 {
		return resp.Encode("PONG", true)
	}
	return resp.Encode(args[0], false)
}

func (c *CommandHandler) cmdCOMMAND() []byte {
	return resp.Ok
}

func (c *CommandHandler) cmdINFO() []byte {
	stats := c.store.Stats()
	hubStats := c.hub.Stats()
	info := fmt.Sprintf(
		"# Server\r\n# Keyspace\r\ntotal_keys:%d\r\nstring_keys:%d\r\nhash_keys:%d\r\nlist_keys:%d\r\nset_keys:%d\r\n\r\n# PubSub\r\nchannels:%d\r\nsubscribers:%d\r\n",
		stats.TotalKeys, stats.StringKeys, stats.HashKeys, stats.ListKeys, stats.SetKeys,
		hubStats.Channels, hubStats.Subscribers,
	)
	return resp.Encode(info, false)
}
