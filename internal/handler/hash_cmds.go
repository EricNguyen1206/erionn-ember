package handler

import (
	"sort"

	"gomemkv/pkg/resp"
)

func (c *CommandHandler) cmdHSET(args []string) []byte {
	if len(args) < 3 || len(args)%2 == 0 {
		return resp.EncodeError("ERR wrong number of arguments for 'HSET' command")
	}

	key := args[0]
	fields := make(map[string]string, (len(args)-1)/2)
	for i := 1; i < len(args); i += 2 {
		fields[args[i]] = args[i+1]
	}

	added, err := c.store.HSet(key, fields)
	if err != nil {
		return mapStoreError(err)
	}
	return resp.Encode(added, false)
}

func (c *CommandHandler) cmdHGET(args []string) []byte {
	if len(args) != 2 {
		return resp.EncodeError("ERR wrong number of arguments for 'HGET' command")
	}

	value, found, err := c.store.HGet(args[0], args[1])
	if err != nil {
		return mapStoreError(err)
	}
	if !found {
		return resp.Nil
	}
	return resp.Encode(value, false)
}

func (c *CommandHandler) cmdHDEL(args []string) []byte {
	if len(args) < 2 {
		return resp.EncodeError("ERR wrong number of arguments for 'HDEL' command")
	}

	removed, err := c.store.HDel(args[0], args[1:])
	if err != nil {
		return mapStoreError(err)
	}
	return resp.Encode(removed, false)
}

func (c *CommandHandler) cmdHGETALL(args []string) []byte {
	if len(args) != 1 {
		return resp.EncodeError("ERR wrong number of arguments for 'HGETALL' command")
	}

	fields, found, err := c.store.HGetAll(args[0])
	if err != nil {
		return mapStoreError(err)
	}
	if !found {
		return resp.EmptyArray
	}

	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	flat := make([]string, 0, len(fields)*2)
	for _, k := range keys {
		flat = append(flat, k, fields[k])
	}
	return resp.Encode(flat, false)
}
