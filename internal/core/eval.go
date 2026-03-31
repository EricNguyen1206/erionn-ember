package core

import (
	"errors"
	"fmt"
	"io"
)

func cmdPING(args []string) []byte {
	var buf []byte

	if len(args) > 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'PING' command"), false)
	}

	if len(args) == 0 {
		buf = Encode("PONG", true)
	} else {
		buf = Encode(args[0], false)
	}

	return buf
}

// EvalAndResponse is the legacy command evaluator.
// Deprecated: Use cmd_handler.CommandHandler.Execute instead.
func EvalAndResponse(cmd *MemKVCmd, c io.ReadWriter) error {
	var res []byte

	switch cmd.Cmd {
	case "PING":
		res = cmdPING(cmd.Args)
	default:
		return fmt.Errorf("command not found: %s", cmd.Cmd)
	}
	_, err := c.Write(res)
	return err
}
