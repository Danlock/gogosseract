package wasm

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"

	"github.com/danlock/gogosseract/internal/gen"
	embind "github.com/jerbob92/wazero-emscripten-embind"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

//go:embed tesseract-core.wasm
var tesseractWASM []byte

func CompileTesseract(ctx context.Context, waRT wazero.Runtime, embEng embind.Engine) (api.Module, error) {
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
		WithStderr(os.Stderr).
		WithStdout(os.Stdout).
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

func parseTextFromImage(ctx context.Context, tessMod api.Module, embEng embind.Engine, img []byte, langModel []byte) (string, error) {
	logPrefix := "ParseTextFromImage"

	ocrEngine, err := gen.NewClassOCREngine(embEng, ctx)
	if err != nil {
		return "", fmt.Errorf(logPrefix+" gen.NewClassOCREngine %w", err)
	}

	modelPtr, err := WriteBytes(ctx, tessMod, langModel)
	if err != nil {
		return "", fmt.Errorf(logPrefix+" WriteBytes %w", err)
	}
	defer Free(ctx, tessMod, modelPtr)

	ocrErr, err := ocrEngine.LoadModel(ctx, uint32(modelPtr), uint32(len(langModel)), "eng")
	if err != nil || ocrErr != "" {
		return "", fmt.Errorf(logPrefix+" ocrEngine.LoadModel ocrErr (%s) %w", ocrErr, err)
	}

	/*
		Unable to figure out how to register certain types with embind...
		Passing a []byte to C++ expecting a std:vector<unsigned char> results in Cannot call OCREngine.loadRawImage due to unbound types: NSt3__26vectorIhNS_9allocatorIhEEEE
		Passing an int to C++ expecting a const unsigned char * results in a Cannot call OCREngine.loadRawImage due to unbound types: PKh
		Passing an int to C++ expecting a void * results in a Cannot call OCREngine.loadRawImage due to unbound types: Pv
		Passing any kind of raw pointer seems to result in a variety of unbound types errors.
		Passing a string seems to work, although that will incur a Go copy of []byte to string, and then additional embind copies to the WASM memory.
		Instead we just skip over embind altogether and directly call pixReadMem on our previously written bytes. Seems to be as efficient as we can get.
	*/

	imgPtr, err := WriteBytes(ctx, tessMod, img)
	if err != nil {
		return "", fmt.Errorf(logPrefix+" WriteBytes %w", err)
	}
	defer Free(ctx, tessMod, imgPtr)

	results, err := tessMod.ExportedFunction("pixReadMem").Call(ctx, imgPtr, uint64(len(img)))
	if err != nil || len(results) != 1 || results[0] == 0 {
		return "", fmt.Errorf(logPrefix+" pixReadMem results=(%v) %w", results, err)
	}
	pixPtr := results[0]

	ocrErr, err = ocrEngine.LoadImage(ctx, uint32(pixPtr))
	if err != nil || ocrErr != "" {
		return "", fmt.Errorf(logPrefix+" ocrEngine.LoadImage ocrErr=(%v) %w", ocrErr, err)
	}

	parsedText, err := ocrEngine.GetText(ctx, func(progress int32) { slog.Info("message parsing...", "progress", progress) })
	if err != nil {
		return "", fmt.Errorf(logPrefix+" ocrEngine.GetText %w", err)
	}
	return parsedText, nil
}
