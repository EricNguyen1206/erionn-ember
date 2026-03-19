package core

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// RESPReader wraps a bufio.Reader to provide RESP decoding.
type RESPReader struct {
	reader *bufio.Reader
}

// NewRESPReader creates a new RESPReader.
func NewRESPReader(rd io.Reader) *RESPReader {
	return &RESPReader{
		reader: bufio.NewReader(rd),
	}
}

// ReadCommand reads a complete RESP array or an inline command from the stream.
func (r *RESPReader) ReadCommand() (*MemKVCmd, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}

	if len(line) == 0 {
		return nil, errors.New("empty command")
	}

	switch line[0] {
	case '*': // RESP Array
		return r.readArray(line)
	default: // Inline command (e.g., PING)
		return r.readInline(line)
	}
}

// readLine reads until \r\n and returns the line without the terminator.
func (r *RESPReader) readLine() ([]byte, error) {
	line, err := r.reader.ReadSlice('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return nil, errors.New("invalid line terminator")
	}
	return line[:len(line)-2], nil
}

// readArray reads a RESP array and returns it as a MemKVCmd.
func (r *RESPReader) readArray(header []byte) (*MemKVCmd, error) {
	count, err := strconv.Atoi(string(header[1:]))
	if err != nil || count < 0 {
		return nil, fmt.Errorf("invalid array length: %s", header[1:])
	}

	tokens := make([]string, count)
	for i := 0; i < count; i++ {
		val, err := r.readObject()
		if err != nil {
			return nil, err
		}
		// In Redis, commands are transferred as bulk strings within an array.
		// We expect subsequent elements to be bulk strings for command arguments.
		if s, ok := val.(string); ok {
			tokens[i] = s
		} else {
			return nil, errors.New("expected string in command array")
		}
	}

	if len(tokens) == 0 {
		return nil, errors.New("empty command array")
	}

	return &MemKVCmd{
		Cmd:  strings.ToUpper(tokens[0]),
		Args: tokens[1:],
	}, nil
}

// readObject reads exactly one RESP object (string, integer, etc.).
func (r *RESPReader) readObject() (interface{}, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}

	if len(line) == 0 {
		return nil, errors.New("empty object")
	}

	switch line[0] {
	case '+': // Simple String
		return string(line[1:]), nil
	case '-': // Error
		return errors.New(string(line[1:])), nil
	case ':': // Integer
		return strconv.ParseInt(string(line[1:]), 10, 64)
	case '$': // Bulk String
		return r.readBulkString(line)
	case '*': // Array (nested)
		// For commands, we usually don't have nested arrays, but we can support it.
		return r.readArrayRecursive(line)
	default:
		return nil, fmt.Errorf("unknown RESP type: %c", line[0])
	}
}

func (r *RESPReader) readBulkString(header []byte) (string, error) {
	size, err := strconv.Atoi(string(header[1:]))
	if err != nil {
		return "", fmt.Errorf("invalid bulk string size: %s", header[1:])
	}

	if size == -1 {
		return "", nil // Null bulk string
	}

	buf := make([]byte, size)
	_, err = io.ReadFull(r.reader, buf)
	if err != nil {
		return "", err
	}

	// Read trailing \r\n
	if _, err := r.readLine(); err != nil {
		return "", err
	}

	return string(buf), nil
}

// readArrayRecursive handles nested arrays if needed.
func (r *RESPReader) readArrayRecursive(header []byte) (interface{}, error) {
	count, err := strconv.Atoi(string(header[1:]))
	if err != nil || count < 0 {
		return nil, fmt.Errorf("invalid array length: %s", header[1:])
	}

	res := make([]interface{}, count)
	for i := 0; i < count; i++ {
		val, err := r.readObject()
		if err != nil {
			return nil, err
		}
		res[i] = val
	}
	return res, nil
}

// readInline handles plain text commands like "PING\r\n".
func (r *RESPReader) readInline(line []byte) (*MemKVCmd, error) {
	tokens := strings.Fields(string(line))
	if len(tokens) == 0 {
		return nil, errors.New("empty inline command")
	}
	return &MemKVCmd{
		Cmd:  strings.ToUpper(tokens[0]),
		Args: tokens[1:],
	}, nil
}
