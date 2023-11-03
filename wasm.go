package gogosseract

import (
	"context"
	"fmt"
	"os"

	"github.com/danlock/gogosseract/internal/wasm"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// WASMModule enables reuse of the WebAssembly runtime and compilation for running multiple Tesseract clients simultaenously.
type WASMModule struct {
	waRT   wazero.Runtime
	module api.Module
}

type WASMConfig struct {
	wasm.CompileTesseractConfig
}

// NewWASMModule returns a Webassembly runtime including the compiled Tesseract WASM module ready for use.
// Optimally run multiple Tesseract instances by including WASMModule in their TesseractConfig's to prevent needless recompilation.
func NewWASMModule(ctx context.Context, cfg WASMConfig) (_ *WASMModule, err error) {
	logPrefix := "NewWASM"
	w := &WASMModule{
		waRT: wazero.NewRuntime(ctx),
	}

	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}
	if cfg.Stdout == nil {
		cfg.Stdout = os.Stdout
	}

	w.module, err = wasm.CompileTesseract(ctx, w.waRT, cfg.CompileTesseractConfig)
	if err != nil {
		return nil, fmt.Errorf(logPrefix+" %w", err)
	}
	return w, nil
}

// Close closes the Webassembly runtime.
func (w *WASMModule) Close(ctx context.Context) error {
	return w.waRT.Close(ctx)
}
