package wasm

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"log/slog"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

//go:embed tesseract-core.wasm
var tesseractWASM []byte

type CompileTesseractConfig struct {
	// Stderr and Stdout are used to redirect Tesseract's output. If left nil these are set to os.Stdout and os.Stderr. Set to io.Discard to ignore.
	Stderr, Stdout io.Writer
}

func CompileTesseract(ctx context.Context, waRT wazero.Runtime, cfg CompileTesseractConfig) (api.Module, error) {
	logPrefix := "CompileTesseract"
	tessCompiled, err := waRT.CompileModule(ctx, tesseractWASM)
	if err != nil {
		return nil, fmt.Errorf(logPrefix+" waRT.CompileModule %w", err)
	}

	if err := BuildImports(ctx, waRT, tessCompiled); err != nil {
		return nil, fmt.Errorf(logPrefix+" %w", err)
	}
	tessMod, err := waRT.InstantiateModule(ctx, tessCompiled, wazero.NewModuleConfig().
		WithStderr(cfg.Stderr).
		WithStdout(cfg.Stdout).
		WithStartFunctions("_initialize"))
	if err != nil {
		return nil, fmt.Errorf(logPrefix+" waRT.InstantiateModule %w", err)
	}
	logFields := make([]any, 0)
	for f, d := range tessMod.ExportedFunctionDefinitions() {
		logFields = append(logFields, f)
		logFields = append(logFields, d.Name())
		// logFields = append(logFields, fmt.Sprintf("%+v", d))
	}
	slog.Info("exported functions", logFields...)

	return tessMod, nil
}
