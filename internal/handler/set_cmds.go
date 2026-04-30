package handler

import (
	"gomemkv/pkg/resp"
)

func (c *CommandHandler) cmdSADD(args []string) []byte {
	if len(args) < 2 {
		return resp.EncodeError("ERR wrong number of arguments for 'SADD' command")
	}

	added, err := c.store.SAdd(args[0], args[1:])
	if err != nil {
		return mapStoreError(err)
	}
	return resp.Encode(added, false)
}

func (c *CommandHandler) cmdSREM(args []string) []byte {
	if len(args) < 2 {
		return resp.EncodeError("ERR wrong number of arguments for 'SREM' command")
	}

	removed, err := c.store.SRem(args[0], args[1:])
	if err != nil {
		return mapStoreError(err)
	}
	return resp.Encode(removed, false)
}

func (c *CommandHandler) cmdSMEMBERS(args []string) []byte {
	if len(args) != 1 {
		return resp.EncodeError("ERR wrong number of arguments for 'SMEMBERS' command")
	}

	members, found, err := c.store.SMembers(args[0])
	if err != nil {
		return mapStoreError(err)
	}
	if !found {
		return resp.EmptyArray
	}
	return resp.Encode(members, false)
}

func (c *CommandHandler) cmdSISMEMBER(args []string) []byte {
	if len(args) != 2 {
		return resp.EncodeError("ERR wrong number of arguments for 'SISMEMBER' command")
	}

	isMember, err := c.store.SIsMember(args[0], args[1])
	if err != nil {
		return mapStoreError(err)
	}
	if isMember {
		return resp.One
	}
	return resp.Zero
}

func (c *CommandHandler) cmdSCARD(args []string) []byte {
	if len(args) != 1 {
		return resp.EncodeError("ERR wrong number of arguments for 'SCARD' command")
	}

	members, found, err := c.store.SMembers(args[0])
	if err != nil {
		return mapStoreError(err)
	}
	if !found {
		return resp.Zero
	}
	return resp.Encode(len(members), false)
}
