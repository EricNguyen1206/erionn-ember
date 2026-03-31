package cmd_handler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gomemkv/internal/constant"
	"gomemkv/internal/core"
	"gomemkv/internal/pubsub"
	"gomemkv/internal/store"
)

// CommandHandler dispatches MemKVCmd commands to the store and pubsub hub.
type CommandHandler struct {
	store *store.Store
	hub   *pubsub.Hub
}

// New creates a new CommandHandler.
func New(s *store.Store, h *pubsub.Hub) *CommandHandler {
	return &CommandHandler{store: s, hub: h}
}

// Execute runs a command and returns the RESP-encoded response.
func (c *CommandHandler) Execute(cmd *core.MemKVCmd) []byte {
	switch cmd.Cmd {
	// Utility
	case "PING":
		return c.cmdPING(cmd.Args)
	case "COMMAND":
		return c.cmdCOMMAND()
	case "INFO":
		return c.cmdINFO()

	// String
	case "SET":
		return c.cmdSET(cmd.Args)
	case "GET":
		return c.cmdGET(cmd.Args)
	case "DEL":
		return c.cmdDEL(cmd.Args)
	case "EXISTS":
		return c.cmdEXISTS(cmd.Args)
	case "TYPE":
		return c.cmdTYPE(cmd.Args)
	case "EXPIRE":
		return c.cmdEXPIRE(cmd.Args)
	case "TTL":
		return c.cmdTTL(cmd.Args)
	case "INCR":
		return c.cmdINCR(cmd.Args)

	// Hash
	case "HSET":
		return c.cmdHSET(cmd.Args)
	case "HGET":
		return c.cmdHGET(cmd.Args)
	case "HDEL":
		return c.cmdHDEL(cmd.Args)
	case "HGETALL":
		return c.cmdHGETALL(cmd.Args)

	// List
	case "LPUSH":
		return c.cmdLPUSH(cmd.Args)
	case "RPUSH":
		return c.cmdRPUSH(cmd.Args)
	case "LPOP":
		return c.cmdLPOP(cmd.Args)
	case "RPOP":
		return c.cmdRPOP(cmd.Args)
	case "LRANGE":
		return c.cmdLRANGE(cmd.Args)

	// Set
	case "SADD":
		return c.cmdSADD(cmd.Args)
	case "SREM":
		return c.cmdSREM(cmd.Args)
	case "SMEMBERS":
		return c.cmdSMEMBERS(cmd.Args)
	case "SISMEMBER":
		return c.cmdSISMEMBER(cmd.Args)
	case "SCARD":
		return c.cmdSCARD(cmd.Args)

	// Pub/Sub
	case "PUBLISH":
		return c.cmdPUBLISH(cmd.Args)

	default:
		return respError(fmt.Sprintf("ERR unknown command '%s'", cmd.Cmd))
	}
}

// --- Utility commands ---

func (c *CommandHandler) cmdPING(args []string) []byte {
	if len(args) > 1 {
		return respError("ERR wrong number of arguments for 'PING' command")
	}
	if len(args) == 0 {
		return core.Encode("PONG", true)
	}
	return core.Encode(args[0], false)
}

func (c *CommandHandler) cmdCOMMAND() []byte {
	// Minimal response for redis-cli handshake
	return constant.RespOk
}

func (c *CommandHandler) cmdINFO() []byte {
	stats := c.store.Stats()
	hubStats := c.hub.Stats()
	info := fmt.Sprintf(
		"# Server\r\nerion_version:4.0.0\r\n\r\n# Keyspace\r\ntotal_keys:%d\r\nstring_keys:%d\r\nhash_keys:%d\r\nlist_keys:%d\r\nset_keys:%d\r\n\r\n# PubSub\r\nchannels:%d\r\nsubscribers:%d\r\n",
		stats.TotalKeys, stats.StringKeys, stats.HashKeys, stats.ListKeys, stats.SetKeys,
		hubStats.Channels, hubStats.Subscribers,
	)
	return core.Encode(info, false)
}

// --- String commands ---

func (c *CommandHandler) cmdSET(args []string) []byte {
	if len(args) < 2 {
		return respError("ERR wrong number of arguments for 'SET' command")
	}

	key, value := args[0], args[1]
	var ttl time.Duration

	// Parse optional EX/PX
	for i := 2; i < len(args); i++ {
		switch strings.ToUpper(args[i]) {
		case "EX":
			if i+1 >= len(args) {
				return respError("ERR syntax error")
			}
			seconds, err := strconv.ParseInt(args[i+1], 10, 64)
			if err != nil || seconds <= 0 {
				return respError("ERR value is not an integer or out of range")
			}
			ttl = time.Duration(seconds) * time.Second
			i++
		case "PX":
			if i+1 >= len(args) {
				return respError("ERR syntax error")
			}
			millis, err := strconv.ParseInt(args[i+1], 10, 64)
			if err != nil || millis <= 0 {
				return respError("ERR value is not an integer or out of range")
			}
			ttl = time.Duration(millis) * time.Millisecond
			i++
		default:
			return respError("ERR syntax error")
		}
	}

	if err := c.store.SetString(key, value, ttl); err != nil {
		return mapStoreError(err)
	}
	return constant.RespOk
}

func (c *CommandHandler) cmdGET(args []string) []byte {
	if len(args) != 1 {
		return respError("ERR wrong number of arguments for 'GET' command")
	}

	value, found, err := c.store.GetString(args[0])
	if err != nil {
		return mapStoreError(err)
	}
	if !found {
		return constant.RespNil
	}
	return core.Encode(value, false)
}

func (c *CommandHandler) cmdDEL(args []string) []byte {
	if len(args) == 0 {
		return respError("ERR wrong number of arguments for 'DEL' command")
	}

	deleted := 0
	for _, key := range args {
		if c.store.Del(key) {
			deleted++
		}
	}
	return core.Encode(deleted, false)
}

func (c *CommandHandler) cmdEXISTS(args []string) []byte {
	if len(args) == 0 {
		return respError("ERR wrong number of arguments for 'EXISTS' command")
	}

	count := 0
	for _, key := range args {
		if c.store.Exists(key) {
			count++
		}
	}
	return core.Encode(count, false)
}

func (c *CommandHandler) cmdTYPE(args []string) []byte {
	if len(args) != 1 {
		return respError("ERR wrong number of arguments for 'TYPE' command")
	}

	t := c.store.Type(args[0])
	return core.Encode(string(t), true)
}

func (c *CommandHandler) cmdEXPIRE(args []string) []byte {
	if len(args) != 2 {
		return respError("ERR wrong number of arguments for 'EXPIRE' command")
	}

	seconds, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return respError("ERR value is not an integer or out of range")
	}
	if seconds <= 0 {
		return respError("ERR invalid expire time in 'EXPIRE' command")
	}

	if c.store.Expire(args[0], time.Duration(seconds)*time.Second) {
		return constant.RespOne
	}
	return constant.RespZero
}

func (c *CommandHandler) cmdTTL(args []string) []byte {
	if len(args) != 1 {
		return respError("ERR wrong number of arguments for 'TTL' command")
	}

	ttl, hasTTL, exists := c.store.TTL(args[0])
	if !exists {
		return constant.TtlKeyNotExist
	}
	if !hasTTL {
		return constant.TtlKeyExistNoExpire
	}
	return core.Encode(int(ttl.Seconds()), false)
}

func (c *CommandHandler) cmdINCR(args []string) []byte {
	if len(args) != 1 {
		return respError("ERR wrong number of arguments for 'INCR' command")
	}

	key := args[0]
	value, found, err := c.store.GetString(key)
	if err != nil {
		return mapStoreError(err)
	}

	var num int64
	if found {
		num, err = strconv.ParseInt(value, 10, 64)
		if err != nil {
			return respError("ERR value is not an integer or out of range")
		}
	}
	num++

	if err := c.store.SetString(key, strconv.FormatInt(num, 10), 0); err != nil {
		return mapStoreError(err)
	}
	return core.Encode(int(num), false)
}

// --- Hash commands ---

func (c *CommandHandler) cmdHSET(args []string) []byte {
	if len(args) < 3 || len(args)%2 == 0 {
		return respError("ERR wrong number of arguments for 'HSET' command")
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
	return core.Encode(added, false)
}

func (c *CommandHandler) cmdHGET(args []string) []byte {
	if len(args) != 2 {
		return respError("ERR wrong number of arguments for 'HGET' command")
	}

	value, found, err := c.store.HGet(args[0], args[1])
	if err != nil {
		return mapStoreError(err)
	}
	if !found {
		return constant.RespNil
	}
	return core.Encode(value, false)
}

func (c *CommandHandler) cmdHDEL(args []string) []byte {
	if len(args) < 2 {
		return respError("ERR wrong number of arguments for 'HDEL' command")
	}

	removed, err := c.store.HDel(args[0], args[1:])
	if err != nil {
		return mapStoreError(err)
	}
	return core.Encode(removed, false)
}

func (c *CommandHandler) cmdHGETALL(args []string) []byte {
	if len(args) != 1 {
		return respError("ERR wrong number of arguments for 'HGETALL' command")
	}

	fields, found, err := c.store.HGetAll(args[0])
	if err != nil {
		return mapStoreError(err)
	}
	if !found {
		return constant.RespEmptyArray
	}

	// Flatten map to alternating key/value array
	flat := make([]string, 0, len(fields)*2)
	for k, v := range fields {
		flat = append(flat, k, v)
	}
	return core.Encode(flat, false)
}

// --- List commands ---

func (c *CommandHandler) cmdLPUSH(args []string) []byte {
	if len(args) < 2 {
		return respError("ERR wrong number of arguments for 'LPUSH' command")
	}

	length, err := c.store.LPush(args[0], args[1:])
	if err != nil {
		return mapStoreError(err)
	}
	return core.Encode(length, false)
}

func (c *CommandHandler) cmdRPUSH(args []string) []byte {
	if len(args) < 2 {
		return respError("ERR wrong number of arguments for 'RPUSH' command")
	}

	length, err := c.store.RPush(args[0], args[1:])
	if err != nil {
		return mapStoreError(err)
	}
	return core.Encode(length, false)
}

func (c *CommandHandler) cmdLPOP(args []string) []byte {
	if len(args) != 1 {
		return respError("ERR wrong number of arguments for 'LPOP' command")
	}

	value, found, err := c.store.LPop(args[0])
	if err != nil {
		return mapStoreError(err)
	}
	if !found {
		return constant.RespNil
	}
	return core.Encode(value, false)
}

func (c *CommandHandler) cmdRPOP(args []string) []byte {
	if len(args) != 1 {
		return respError("ERR wrong number of arguments for 'RPOP' command")
	}

	value, found, err := c.store.RPop(args[0])
	if err != nil {
		return mapStoreError(err)
	}
	if !found {
		return constant.RespNil
	}
	return core.Encode(value, false)
}

func (c *CommandHandler) cmdLRANGE(args []string) []byte {
	if len(args) != 3 {
		return respError("ERR wrong number of arguments for 'LRANGE' command")
	}

	start, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return respError("ERR value is not an integer or out of range")
	}
	stop, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		return respError("ERR value is not an integer or out of range")
	}

	values, found, err := c.store.LRange(args[0], start, stop)
	if err != nil {
		return mapStoreError(err)
	}
	if !found {
		return constant.RespEmptyArray
	}
	return core.Encode(values, false)
}

// --- Set commands ---

func (c *CommandHandler) cmdSADD(args []string) []byte {
	if len(args) < 2 {
		return respError("ERR wrong number of arguments for 'SADD' command")
	}

	added, err := c.store.SAdd(args[0], args[1:])
	if err != nil {
		return mapStoreError(err)
	}
	return core.Encode(added, false)
}

func (c *CommandHandler) cmdSREM(args []string) []byte {
	if len(args) < 2 {
		return respError("ERR wrong number of arguments for 'SREM' command")
	}

	removed, err := c.store.SRem(args[0], args[1:])
	if err != nil {
		return mapStoreError(err)
	}
	return core.Encode(removed, false)
}

func (c *CommandHandler) cmdSMEMBERS(args []string) []byte {
	if len(args) != 1 {
		return respError("ERR wrong number of arguments for 'SMEMBERS' command")
	}

	members, found, err := c.store.SMembers(args[0])
	if err != nil {
		return mapStoreError(err)
	}
	if !found {
		return constant.RespEmptyArray
	}
	return core.Encode(members, false)
}

func (c *CommandHandler) cmdSISMEMBER(args []string) []byte {
	if len(args) != 2 {
		return respError("ERR wrong number of arguments for 'SISMEMBER' command")
	}

	isMember, err := c.store.SIsMember(args[0], args[1])
	if err != nil {
		return mapStoreError(err)
	}
	if isMember {
		return constant.RespOne
	}
	return constant.RespZero
}

func (c *CommandHandler) cmdSCARD(args []string) []byte {
	if len(args) != 1 {
		return respError("ERR wrong number of arguments for 'SCARD' command")
	}

	members, found, err := c.store.SMembers(args[0])
	if err != nil {
		return mapStoreError(err)
	}
	if !found {
		return constant.RespZero
	}
	return core.Encode(len(members), false)
}

// --- Pub/Sub commands ---

func (c *CommandHandler) cmdPUBLISH(args []string) []byte {
	if len(args) != 2 {
		return respError("ERR wrong number of arguments for 'PUBLISH' command")
	}

	delivered := c.hub.Publish(args[0], []byte(args[1]))
	return core.Encode(delivered, false)
}

// --- Helpers ---

func respError(msg string) []byte {
	return []byte(fmt.Sprintf("-%s\r\n", msg))
}

func mapStoreError(err error) []byte {
	if errors.Is(err, store.ErrWrongType) {
		return respError("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	return respError(fmt.Sprintf("ERR %s", err.Error()))
}
