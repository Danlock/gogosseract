package wasm

import (
	"bytes"
	"context"
	_ "embed"
	"io"
	"testing"

	"github.com/danlock/pkg/test"
	embind "github.com/jerbob92/wazero-emscripten-embind"
	"github.com/tetratelabs/wazero"
)

func TestCompileTesseract(t *testing.T) {
	embEng := embind.CreateEngine(embind.NewConfig())
	ctx := embEng.Attach(context.Background())
	waRT := wazero.NewRuntime(ctx)
	defer waRT.Close(ctx)

	_, err := CompileTesseract(ctx, waRT, embEng, CompileConfig{})
	if err != nil {
		t.Fatalf("CompileTesseract() error = %v", err)
	}
}

func TestGetReaderSize(t *testing.T) {
	ctx := context.Background()
	wantedLen := len(tesseractWASM)
	var buf io.Reader = bytes.NewBuffer(tesseractWASM)
	len, err := GetReaderSize(ctx, &buf)
	test.FailOnError(t, err)
	if len != uint32(wantedLen) {
		t.Fatalf("GetReaderSize gave %d wanted %d", len, wantedLen)
	}

	if _, err = GetReaderSize(ctx, nil); err == nil {
		t.Fatalf("GetReaderSize should have errored")
	}

	var emptyBuf io.Reader = bytes.NewBuffer([]byte{})
	if _, err = GetReaderSize(ctx, &emptyBuf); err == nil {
		t.Fatalf("GetReaderSize should have errored")
	}
}
