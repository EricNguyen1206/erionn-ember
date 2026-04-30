package handler

import (
	"strconv"
	"strings"
	"time"

	"gomemkv/pkg/resp"
)

func (c *CommandHandler) cmdSET(args []string) []byte {
	if len(args) < 2 {
		return resp.EncodeError("ERR wrong number of arguments for 'SET' command")
	}

	key, value := args[0], args[1]
	var ttl time.Duration

	for i := 2; i < len(args); i++ {
		switch strings.ToUpper(args[i]) {
		case "EX":
			if i+1 >= len(args) {
				return resp.EncodeError("ERR syntax error")
			}
			seconds, err := strconv.ParseInt(args[i+1], 10, 64)
			if err != nil || seconds <= 0 {
				return resp.EncodeError("ERR value is not an integer or out of range")
			}
			ttl = time.Duration(seconds) * time.Second
			i++
		case "PX":
			if i+1 >= len(args) {
				return resp.EncodeError("ERR syntax error")
			}
			millis, err := strconv.ParseInt(args[i+1], 10, 64)
			if err != nil || millis <= 0 {
				return resp.EncodeError("ERR value is not an integer or out of range")
			}
			ttl = time.Duration(millis) * time.Millisecond
			i++
		default:
			return resp.EncodeError("ERR syntax error")
		}
	}

	if err := c.store.SetString(key, value, ttl); err != nil {
		return mapStoreError(err)
	}
	return resp.Ok
}

func (c *CommandHandler) cmdSETEX(args []string) []byte {
	if len(args) != 3 {
		return resp.EncodeError("ERR wrong number of arguments for 'SETEX' command")
	}

	seconds, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil || seconds <= 0 {
		return resp.EncodeError("ERR invalid expire time in 'SETEX' command")
	}

	if err := c.store.SetString(args[0], args[2], time.Duration(seconds)*time.Second); err != nil {
		return mapStoreError(err)
	}
	return resp.Ok
}

func (c *CommandHandler) cmdGET(args []string) []byte {
	if len(args) != 1 {
		return resp.EncodeError("ERR wrong number of arguments for 'GET' command")
	}

	value, found, err := c.store.GetString(args[0])
	if err != nil {
		return mapStoreError(err)
	}
	if !found {
		return resp.Nil
	}
	return resp.Encode(value, false)
}

func (c *CommandHandler) cmdDEL(args []string) []byte {
	if len(args) == 0 {
		return resp.EncodeError("ERR wrong number of arguments for 'DEL' command")
	}

	deleted := 0
	for _, key := range args {
		if c.store.Del(key) {
			deleted++
		}
	}
	return resp.Encode(deleted, false)
}

func (c *CommandHandler) cmdEXISTS(args []string) []byte {
	if len(args) == 0 {
		return resp.EncodeError("ERR wrong number of arguments for 'EXISTS' command")
	}

	count := 0
	for _, key := range args {
		if c.store.Exists(key) {
			count++
		}
	}
	return resp.Encode(count, false)
}

func (c *CommandHandler) cmdTYPE(args []string) []byte {
	if len(args) != 1 {
		return resp.EncodeError("ERR wrong number of arguments for 'TYPE' command")
	}

	t := c.store.Type(args[0])
	return resp.Encode(string(t), true)
}

func (c *CommandHandler) cmdEXPIRE(args []string) []byte {
	if len(args) != 2 {
		return resp.EncodeError("ERR wrong number of arguments for 'EXPIRE' command")
	}

	seconds, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return resp.EncodeError("ERR value is not an integer or out of range")
	}
	if seconds <= 0 {
		return resp.EncodeError("ERR invalid expire time in 'EXPIRE' command")
	}

	if c.store.Expire(args[0], time.Duration(seconds)*time.Second) {
		return resp.One
	}
	return resp.Zero
}

func (c *CommandHandler) cmdTTL(args []string) []byte {
	if len(args) != 1 {
		return resp.EncodeError("ERR wrong number of arguments for 'TTL' command")
	}

	ttl, hasTTL, exists := c.store.TTL(args[0])
	if !exists {
		return resp.TTLKeyNotExist
	}
	if !hasTTL {
		return resp.TTLKeyNoExpire
	}
	return resp.Encode(int(ttl.Seconds()), false)
}

func (c *CommandHandler) cmdINCR(args []string) []byte {
	if len(args) != 1 {
		return resp.EncodeError("ERR wrong number of arguments for 'INCR' command")
	}

	num, err := c.store.IncrString(args[0])
	if err != nil {
		return mapStoreError(err)
	}
	return resp.Encode(int(num), false)
}
