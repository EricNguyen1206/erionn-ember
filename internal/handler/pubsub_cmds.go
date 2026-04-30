package handler

import (
	"gomemkv/pkg/resp"
)

func (c *CommandHandler) cmdPUBLISH(args []string) []byte {
	if len(args) != 2 {
		return resp.EncodeError("ERR wrong number of arguments for 'PUBLISH' command")
	}

	delivered := c.hub.Publish(args[0], []byte(args[1]))
	return resp.Encode(delivered, false)
}
