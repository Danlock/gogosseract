package wasm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"

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

// ReadAndFreeString frees the read string from WASM's memory, since Go copied it anyway.
func ReadAndFreeString[T constraints.Integer](ctx context.Context, mod api.Module, rawStrPtr T) string {
	str := ReadString(mod.Memory(), rawStrPtr)
	Free(ctx, mod, rawStrPtr)
	return str
}

// WriteString malloc's a C style string within the WASM modules memory. Remember to defer/call Free.
func WriteString(ctx context.Context, mod api.Module, str string) (uint64, error) {
	results, err := mod.ExportedFunction("malloc").Call(ctx, uint64(len(str)+1))
	if err != nil || len(results) != 1 {
		return 0, fmt.Errorf("wasm.AllocateString _malloc results %v err %w", results, err)
	}
	strPtr := uint32(results[0])
	if !mod.Memory().WriteString(strPtr, str) {
		return 0, fmt.Errorf("wasm.AllocateString WriteString failed for %s", str)
	}
	if !mod.Memory().WriteByte(strPtr+uint32(len(str)), 0) {
		return 0, fmt.Errorf("wasm.AllocateString WriteByte 0 failed")
	}
	return results[0], nil
}

func Free[T constraints.Integer](ctx context.Context, mod api.Module, ptr T) {
	_, err := mod.ExportedFunction("free").Call(ctx, uint64(ptr))
	if err != nil {
		panic(fmt.Errorf("wasm.Free err %w", err))
	}
}

type readerWithLen interface {
	io.Reader
	Len() int
}

var _ = readerWithLen(new(bytes.Buffer))

// GetReaderSize attempts to optimally determine an io.Reader's size, with special support for bytes.Buffer like objects,
// or os.File like objects. Takes a pointer to the reader in case it needs to replace it with a copy (worst case).
func GetReaderSize(ctx context.Context, readerPtr *io.Reader) (uint32, error) {
	logPrefix := "GetReaderSize"
	var size uint64
	if readerPtr == nil {
		return 0, fmt.Errorf(logPrefix + " readerPtr nil")
	}

	reader := *readerPtr
	switch src := reader.(type) {
	case readerWithLen:
		size = uint64(src.Len())
	case io.Seeker:
		// This case covers os.File while being friendly to any other io.ReadSeeker's
		streamLen, err := src.Seek(0, io.SeekEnd)
		if err != nil {
			return 0, fmt.Errorf(logPrefix+" io.Seeker.Seek SeekEnd %w", err)
		}
		// Seek back to the beginning so we can read it all
		if _, err := src.Seek(0, io.SeekStart); err != nil {
			return 0, fmt.Errorf(logPrefix+" io.Seeker.Seek SeekStart %w", err)
		}
		size = uint64(streamLen)
	case nil:
		return 0, fmt.Errorf(logPrefix + " got nil io.Reader")
	default:
		// We have failed to avoid copying the data into memory to get the size...
		// git commit sudoku
		buf := new(bytes.Buffer)
		bufLen, err := io.Copy(buf, reader)
		if err != nil {
			return 0, fmt.Errorf(logPrefix+" io.Copy %w", err)
		}
		// reader's been read, point it at buf now.
		*readerPtr = buf
		size = uint64(bufLen)
	}

	if size > math.MaxUint32 {
		return 0, fmt.Errorf(logPrefix+" io.Reader size (%d) too large", size)
	} else if size == 0 {
		return 0, fmt.Errorf(logPrefix + " io.Reader was empty")
	}
	return uint32(size), nil
}
