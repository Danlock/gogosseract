package wasm

import (
	"bytes"
	"context"
	"io"
	"math"

	"github.com/danlock/pkg/errors"
	"github.com/tetratelabs/wazero/api"
	"golang.org/x/exp/constraints"
)

// ReadString reads from the provided pointer until we reach a 0.
// If 0 is not found, returns an empty string.
// If WASM doesn't need it afterwards, use ReadAndFreeString instead.
func ReadString[T constraints.Integer](mem api.Memory, rawStrPtr T) string {
	strPtr := uint32(rawStrPtr)
	str, _ := mem.Read(strPtr, mem.Size()-strPtr)
	strEnd := bytes.IndexByte(str, 0)
	if strEnd == -1 {
		return ""
	}
	return string(str[:strEnd])
}

type readerWithLen interface {
	io.Reader
	Len() int
}

var _ = readerWithLen(new(bytes.Buffer))

// GetReaderSize attempts to optimally determine an io.Reader's size, with special support for bytes.Buffer like objects,
// or os.File like objects. Takes a pointer to the reader in case it needs to replace it with a copy (worst case).
func GetReaderSize(ctx context.Context, readerPtr *io.Reader) (uint32, error) {
	var size uint64
	if readerPtr == nil {
		return 0, errors.Errorf("readerPtr nil")
	}

	reader := *readerPtr
	switch src := reader.(type) {
	case readerWithLen:
		size = uint64(src.Len())
	case io.Seeker:
		// This case covers os.File while being friendly to any other io.ReadSeeker's
		streamLen, err := src.Seek(0, io.SeekEnd)
		if err != nil {
			return 0, errors.Errorf("io.Seeker.Seek SeekEnd %w", err)
		}
		// Seek back to the beginning so we can read it all
		if _, err := src.Seek(0, io.SeekStart); err != nil {
			return 0, errors.Errorf("io.Seeker.Seek SeekStart %w", err)
		}
		size = uint64(streamLen)
	case nil:
		return 0, errors.Errorf("got nil io.Reader")
	default:
		// We have failed to avoid copying the data into memory to get the size...
		// git commit sudoku
		buf := new(bytes.Buffer)
		bufLen, err := io.Copy(buf, reader)
		if err != nil {
			return 0, errors.Errorf("io.Copy %w", err)
		}
		// reader's been read, point it at buf now.
		*readerPtr = buf
		size = uint64(bufLen)
	}

	if size > math.MaxUint32 {
		return 0, errors.Errorf("io.Reader size (%d) too large", size)
	} else if size == 0 {
		return 0, errors.Errorf("io.Reader was empty")
	}
	return uint32(size), nil
}
