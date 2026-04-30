package resp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Reader struct {
	reader *bufio.Reader
}

func NewReader(rd io.Reader) *Reader {
	return &Reader{
		reader: bufio.NewReader(rd),
	}
}

func (r *Reader) ReadCommand() (*Command, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}

	if len(line) == 0 {
		return nil, errors.New("empty command")
	}

	switch line[0] {
	case '*':
		return r.readArray(line)
	default:
		return r.readInline(line)
	}
}

func (r *Reader) readLine() ([]byte, error) {
	line, err := r.reader.ReadSlice('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return nil, errors.New("invalid line terminator")
	}
	return line[:len(line)-2], nil
}

func (r *Reader) readArray(header []byte) (*Command, error) {
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
		if s, ok := val.(string); ok {
			tokens[i] = s
		} else {
			return nil, errors.New("expected string in command array")
		}
	}

	if len(tokens) == 0 {
		return nil, errors.New("empty command array")
	}

	return &Command{
		Cmd:  strings.ToUpper(tokens[0]),
		Args: tokens[1:],
	}, nil
}

func (r *Reader) readObject() (interface{}, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}

	if len(line) == 0 {
		return nil, errors.New("empty object")
	}

	switch line[0] {
	case '+':
		return string(line[1:]), nil
	case '-':
		return errors.New(string(line[1:])), nil
	case ':':
		return strconv.ParseInt(string(line[1:]), 10, 64)
	case '$':
		return r.readBulkString(line)
	case '*':
		return r.readArrayRecursive(line)
	default:
		return nil, fmt.Errorf("unknown RESP type: %c", line[0])
	}
}

func (r *Reader) readBulkString(header []byte) (string, error) {
	size, err := strconv.Atoi(string(header[1:]))
	if err != nil {
		return "", fmt.Errorf("invalid bulk string size: %s", header[1:])
	}

	if size == -1 {
		return "", nil
	}

	buf := make([]byte, size)
	if _, err := io.ReadFull(r.reader, buf); err != nil {
		return "", err
	}

	if _, err := r.readLine(); err != nil {
		return "", err
	}

	return string(buf), nil
}

func (r *Reader) readArrayRecursive(header []byte) (interface{}, error) {
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

func (r *Reader) readInline(line []byte) (*Command, error) {
	tokens := strings.Fields(string(line))
	if len(tokens) == 0 {
		return nil, errors.New("empty inline command")
	}
	return &Command{
		Cmd:  strings.ToUpper(tokens[0]),
		Args: tokens[1:],
	}, nil
}
