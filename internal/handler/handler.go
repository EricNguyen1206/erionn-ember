package handler

import (
	"errors"
	"fmt"

	"gomemkv/internal/pubsub"
	"gomemkv/pkg/resp"
	"gomemkv/internal/store"
)

type CommandHandler struct {
	store *store.Store
	hub   *pubsub.Hub
}

func New(s *store.Store, h *pubsub.Hub) *CommandHandler {
	return &CommandHandler{store: s, hub: h}
}

func (c *CommandHandler) Execute(cmd *resp.Command) []byte {
	switch cmd.Cmd {
	case "PING":
		return c.cmdPING(cmd.Args)
	case "COMMAND":
		return c.cmdCOMMAND()
	case "INFO":
		return c.cmdINFO()

	case "SET":
		return c.cmdSET(cmd.Args)
	case "SETEX":
		return c.cmdSETEX(cmd.Args)
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

	case "HSET":
		return c.cmdHSET(cmd.Args)
	case "HGET":
		return c.cmdHGET(cmd.Args)
	case "HDEL":
		return c.cmdHDEL(cmd.Args)
	case "HGETALL":
		return c.cmdHGETALL(cmd.Args)

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

	case "ZADD":
		return c.cmdZADD(cmd.Args)
	case "ZCARD":
		return c.cmdZCARD(cmd.Args)
	case "ZRANGE":
		return c.cmdZRANGE(cmd.Args)
	case "ZREMRANGEBYSCORE":
		return c.cmdZREMRANGEBYSCORE(cmd.Args)

	case "PUBLISH":
		return c.cmdPUBLISH(cmd.Args)

	default:
		return resp.EncodeError(fmt.Sprintf("ERR unknown command '%s'", cmd.Cmd))
	}
}

func mapStoreError(err error) []byte {
	if errors.Is(err, store.ErrWrongType) {
		return resp.EncodeError("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	return resp.EncodeError(fmt.Sprintf("ERR %s", err.Error()))
}
