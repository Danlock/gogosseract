package gogosseract

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/danlock/gogosseract/internal/wasm"
	embind "github.com/jerbob92/wazero-emscripten-embind"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// WASMModule enables reuse of the WebAssembly runtime and compilation for running multiple Tesseract clients simultaenously.
type WASMModule struct {
	waRT         wazero.Runtime
	module       api.Module
	embindEngine embind.Engine
}

type WASMConfig struct {
	// Stderr and Stdout are used to redirect Tesseract's output. If left nil these are set to os.Stdout and os.Stderr. Set to io.Discard to turn off.
	Stderr, Stdout io.Writer
}

// NewWASM returns a Webassembly runtime including the compiled Tesseract WASM ready for use.
// When running multiple Tesseract instances include WASMModule in their TesseractConfig so they don't have to recompile needlessly.
func NewWASM(ctx context.Context, cfg WASMConfig) (_ *WASMModule, err error) {
	logPrefix := "NewWASM"
	w := &WASMModule{
		waRT:         wazero.NewRuntime(ctx),
		embindEngine: embind.CreateEngine(embind.NewConfig()),
	}
	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}
	if cfg.Stdout == nil {
		cfg.Stdout = os.Stdout
	}

	ctx = w.embindEngine.Attach(ctx)
	w.module, err = wasm.CompileTesseract(ctx, w.waRT, w.embindEngine, wasm.CompileTesseractConfig{
		Stderr: cfg.Stderr, Stdout: cfg.Stdout,
	})
	if err != nil {
		return nil, fmt.Errorf(logPrefix+" wasm.CompileTesseract %w", err)
	}

	return w, nil
}

// Close closes the Webassembly runtime.
func (w *WASMModule) Close(ctx context.Context) error {
	return w.waRT.Close(ctx)
}
