package handler

import (
	"strconv"

	"gomemkv/pkg/resp"
)

func (c *CommandHandler) cmdZADD(args []string) []byte {
	// ZADD key score member [score member ...]
	if len(args) < 3 || len(args)%2 == 0 {
		return resp.EncodeError("ERR wrong number of arguments for 'ZADD' command")
	}

	key := args[0]
	members := make(map[string]float64, (len(args)-1)/2)
	for i := 1; i < len(args); i += 2 {
		score, err := strconv.ParseFloat(args[i], 64)
		if err != nil {
			return resp.EncodeError("ERR value is not a valid float")
		}
		members[args[i+1]] = score
	}

	added, err := c.store.ZAdd(key, members)
	if err != nil {
		return mapStoreError(err)
	}
	return resp.Encode(added, false)
}

func (c *CommandHandler) cmdZCARD(args []string) []byte {
	if len(args) != 1 {
		return resp.EncodeError("ERR wrong number of arguments for 'ZCARD' command")
	}

	count, err := c.store.ZCard(args[0])
	if err != nil {
		return mapStoreError(err)
	}
	return resp.Encode(count, false)
}

func (c *CommandHandler) cmdZRANGE(args []string) []byte {
	// ZRANGE key start stop
	if len(args) != 3 {
		return resp.EncodeError("ERR wrong number of arguments for 'ZRANGE' command")
	}

	start, err := strconv.Atoi(args[1])
	if err != nil {
		return resp.EncodeError("ERR value is not an integer or out of range")
	}
	stop, err := strconv.Atoi(args[2])
	if err != nil {
		return resp.EncodeError("ERR value is not an integer or out of range")
	}

	members, found, err := c.store.ZRange(args[0], start, stop)
	if err != nil {
		return mapStoreError(err)
	}
	if !found {
		return resp.EmptyArray
	}
	return resp.Encode(members, false)
}

func (c *CommandHandler) cmdZREMRANGEBYSCORE(args []string) []byte {
	// ZREMRANGEBYSCORE key min max
	if len(args) != 3 {
		return resp.EncodeError("ERR wrong number of arguments for 'ZREMRANGEBYSCORE' command")
	}

	min, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return resp.EncodeError("ERR min is not a float")
	}
	max, err := strconv.ParseFloat(args[2], 64)
	if err != nil {
		return resp.EncodeError("ERR max is not a float")
	}

	removed, err := c.store.ZRemRangeByScore(args[0], min, max)
	if err != nil {
		return mapStoreError(err)
	}
	return resp.Encode(removed, false)
}
