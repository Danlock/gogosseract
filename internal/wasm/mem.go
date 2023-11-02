package wasm

import (
	"bytes"
	"context"
	"fmt"

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

// WriteBytes malloc's bytes within the WASM modules memory, null terminated so it allocates +1 the inputs size. Remember to defer/call Free.
func WriteBytes(ctx context.Context, mod api.Module, buf []byte) (uint64, error) {
	results, err := mod.ExportedFunction("malloc").Call(ctx, uint64(len(buf)+1))
	if err != nil || len(results) != 1 {
		return 0, fmt.Errorf("wasm.AllocateBytes _malloc results %v err %w", results, err)
	}
	ptr := uint32(results[0])
	if !mod.Memory().Write(ptr, buf) {
		return 0, fmt.Errorf("wasm.AllocateBytes Write failed for %d bytes", len(buf))
	}
	if !mod.Memory().WriteByte(ptr+uint32(len(buf)), 0) {
		return 0, fmt.Errorf("wasm.AllocateBytes WriteByte 0 failed")
	}
	return results[0], nil
}

func Free[T constraints.Integer](ctx context.Context, mod api.Module, ptr T) {
	_, err := mod.ExportedFunction("free").Call(ctx, uint64(ptr))
	if err != nil {
		panic(fmt.Errorf("wasm.Free err %w", err))
	}
}
