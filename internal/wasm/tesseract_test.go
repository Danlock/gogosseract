package wasm

import (
	"context"
	_ "embed"
	"testing"

	embind "github.com/jerbob92/wazero-emscripten-embind"
	"github.com/tetratelabs/wazero"
)

func TestCompileTesseract(t *testing.T) {
	embEng := embind.CreateEngine(embind.NewConfig())
	ctx := embEng.Attach(context.Background())
	waRT := wazero.NewRuntime(ctx)
	defer waRT.Close(ctx)

	_, err := CompileTesseract(ctx, waRT, embEng, CompileTesseractConfig{})
	if err != nil {
		t.Fatalf("CompileTesseract() error = %v", err)
	}
}
