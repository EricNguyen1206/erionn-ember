package resp

import (
	"bytes"
	"fmt"
)

const CRLF = "\r\n"

var (
	Nil            = []byte("$-1\r\n")
	Ok             = []byte("+OK\r\n")
	Zero           = []byte(":0\r\n")
	One            = []byte(":1\r\n")
	EmptyArray     = []byte("*0\r\n")
	TTLKeyNotExist = []byte(":-2\r\n")
	TTLKeyNoExpire = []byte(":-1\r\n")
)

type Command struct {
	Cmd  string
	Args []string
}

func Encode(value interface{}, isSimpleString bool) []byte {
	switch v := value.(type) {
	case string:
		if isSimpleString {
			return []byte(fmt.Sprintf("+%s%s", v, CRLF))
		}
		return []byte(fmt.Sprintf("$%d%s%s%s", len(v), CRLF, v, CRLF))
	case int64, int32, int16, int8, int:
		return []byte(fmt.Sprintf(":%d\r\n", v))
	case error:
		return []byte(fmt.Sprintf("-%s\r\n", v))
	case []string:
		return encodeStringArray(value.([]string))
	case [][]string:
		var b []byte
		buf := bytes.NewBuffer(b)
		for _, sa := range value.([][]string) {
			buf.Write(encodeStringArray(sa))
		}
		return []byte(fmt.Sprintf("*%d\r\n%s", len(value.([][]string)), buf.Bytes()))
	case []interface{}:
		var b []byte
		buf := bytes.NewBuffer(b)
		for _, x := range value.([]interface{}) {
			buf.Write(Encode(x, false))
		}
		return []byte(fmt.Sprintf("*%d\r\n%s", len(value.([]interface{})), buf.Bytes()))
	case []int:
		var b []byte
		buf := bytes.NewBuffer(b)
		for _, n := range value.([]int) {
			buf.Write([]byte(fmt.Sprintf("%d|", n)))
		}
		return []byte(fmt.Sprintf("@%s", buf.Bytes()))
	default:
		return Nil
	}
}

func EncodeError(msg string) []byte {
	return []byte(fmt.Sprintf("-%s\r\n", msg))
}

func encodeString(s string) []byte {
	return []byte(fmt.Sprintf("$%d\r\n%s\r\n", len(s), s))
}

func encodeStringArray(sa []string) []byte {
	var b []byte
	buf := bytes.NewBuffer(b)
	for _, s := range sa {
		buf.Write(encodeString(s))
	}
	return []byte(fmt.Sprintf("*%d\r\n%s", len(sa), buf.Bytes()))
}
