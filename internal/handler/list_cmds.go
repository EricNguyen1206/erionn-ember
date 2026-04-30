package handler

import (
	"strconv"

	"gomemkv/pkg/resp"
)

func (c *CommandHandler) cmdLPUSH(args []string) []byte {
	if len(args) < 2 {
		return resp.EncodeError("ERR wrong number of arguments for 'LPUSH' command")
	}

	length, err := c.store.LPush(args[0], args[1:])
	if err != nil {
		return mapStoreError(err)
	}
	return resp.Encode(length, false)
}

func (c *CommandHandler) cmdRPUSH(args []string) []byte {
	if len(args) < 2 {
		return resp.EncodeError("ERR wrong number of arguments for 'RPUSH' command")
	}

	length, err := c.store.RPush(args[0], args[1:])
	if err != nil {
		return mapStoreError(err)
	}
	return resp.Encode(length, false)
}

func (c *CommandHandler) cmdLPOP(args []string) []byte {
	if len(args) != 1 {
		return resp.EncodeError("ERR wrong number of arguments for 'LPOP' command")
	}

	value, found, err := c.store.LPop(args[0])
	if err != nil {
		return mapStoreError(err)
	}
	if !found {
		return resp.Nil
	}
	return resp.Encode(value, false)
}

func (c *CommandHandler) cmdRPOP(args []string) []byte {
	if len(args) != 1 {
		return resp.EncodeError("ERR wrong number of arguments for 'RPOP' command")
	}

	value, found, err := c.store.RPop(args[0])
	if err != nil {
		return mapStoreError(err)
	}
	if !found {
		return resp.Nil
	}
	return resp.Encode(value, false)
}

func (c *CommandHandler) cmdLRANGE(args []string) []byte {
	if len(args) != 3 {
		return resp.EncodeError("ERR wrong number of arguments for 'LRANGE' command")
	}

	start, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return resp.EncodeError("ERR value is not an integer or out of range")
	}
	stop, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		return resp.EncodeError("ERR value is not an integer or out of range")
	}

	values, found, err := c.store.LRange(args[0], start, stop)
	if err != nil {
		return mapStoreError(err)
	}
	if !found {
		return resp.EmptyArray
	}
	return resp.Encode(values, false)
}
