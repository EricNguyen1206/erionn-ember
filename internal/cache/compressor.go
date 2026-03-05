package cache

import (
	"fmt"

	"github.com/pierrec/lz4/v4"
)

// Compressor compresses/decompresses text using LZ4.
// Prefix byte: 0x01 = compressed, 0x00 = stored as-is.
type Compressor struct{}

func NewCompressor() *Compressor { return &Compressor{} }

func (c *Compressor) Compress(text string) []byte {
	src := []byte(text)
	dst := make([]byte, lz4.CompressBlockBound(len(src)))
	n, err := lz4.CompressBlock(src, dst, nil)
	if err != nil || n == 0 {
		return append([]byte{0x00}, src...)
	}
	return append([]byte{0x01}, dst[:n]...)
}

func (c *Compressor) Decompress(data []byte, originalSize int) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	if data[0] == 0x00 {
		return string(data[1:]), nil
	}
	dst := make([]byte, originalSize)
	n, err := lz4.UncompressBlock(data[1:], dst)
	if err != nil {
		return "", fmt.Errorf("lz4 decompress: %w", err)
	}
	return string(dst[:n]), nil
}
