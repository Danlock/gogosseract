package wasm

import (
	"context"
	_ "embed"
	"fmt"
	"io"

	"github.com/danlock/gogosseract/internal/gen"
	embind "github.com/jerbob92/wazero-emscripten-embind"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

type CompileConfig struct {
	// Stderr and Stdout enable redirection of any logs. If left nil they point at os.Stderr and os.Stdout. Turn off by setting them to io.Discard
	Stderr, Stdout io.Writer
}

func CompileTesseract(ctx context.Context, waRT wazero.Runtime, embEng embind.Engine, cfg CompileConfig) (api.Module, error) {
	logPrefix := "CompileTesseract"
	tessCompiled, err := waRT.CompileModule(ctx, tesseractWASM)
	if err != nil {
		return nil, fmt.Errorf(logPrefix+" waRT.CompileModule %w", err)
	}

	// var imports []any
	// for _, f := range tessCompiled.ImportedFunctions() {
	// 	if strings.Contains(f.Name(), "_embind") || strings.Contains(f.Name(), "_emval") ||
	// 		strings.Contains(f.Name(), "__wasi") || strings.Contains(f.Name(), "invoke") {
	// 		continue
	// 	}
	// 	imports = append(imports, f.Name())
	// 	imports = append(imports, "" /* fmt.Sprintf("%+v", f)*/)
	// }
	// slog.Info("tesseract wasm imports", imports...)

	if err := BuildImports(ctx, waRT, embEng, tessCompiled); err != nil {
		return nil, fmt.Errorf(logPrefix+" BuildImports %w", err)
	}
	tessMod, err := waRT.InstantiateModule(ctx, tessCompiled, wazero.NewModuleConfig().
		WithStderr(cfg.Stderr).
		WithStdout(cfg.Stdout).
		WithStartFunctions("_initialize"))
	if err != nil {
		return nil, fmt.Errorf(logPrefix+" waRT.InstantiateModule %w", err)
	}

	// logFields := make([]any, 0)
	// for _, s := range embEng.GetSymbols() {
	// 	logFields = append(logFields, s.Symbol())
	// 	returns := fmt.Sprintf("ret:%s.%s", s.ReturnType().Name(), s.ReturnType().Type())
	// 	var params string
	// 	for _, p := range s.ArgumentTypes() {
	// 		params += fmt.Sprintf("%s.%s,", p.Name(), p.Type())
	// 	}
	// 	logFields = append(logFields, params+returns)
	// }
	// slog.Info("exported embind symbols", logFields...)

	// logFields := make([]any, 0)
	// for f, d := range tessMod.ExportedFunctionDefinitions() {
	// 	logFields = append(logFields, f)
	// 	logFields = append(logFields, d.Name())
	// 	// logFields = append(logFields, fmt.Sprintf("%+v", d))
	// }
	// slog.Info("exported functions", logFields...)
	return tessMod, gen.Attach(embEng)
}
