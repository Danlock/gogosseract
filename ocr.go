package gogosseract

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/danlock/gogosseract/internal/gen"
	"github.com/danlock/gogosseract/internal/wasm"
	embind "github.com/jerbob92/wazero-emscripten-embind"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

type Config struct {
	wasm.CompileConfig
	// Languages Tesseract scans for. Defaults to "eng".
	Language string
	// Training Data Tesseract uses. Required. Must support the provided language. https://github.com/tesseract-ocr/tessdata_fast for more details.
	TrainingData io.Reader
	// Variables are optionally passed into Tesseract as variable config options. Some options are listed at http://www.sk-spell.sk.cx/tesseract-ocr-parameters-in-302-version
	Variables map[string]string
	// WASMCache is an optional wazero.CompilationCache used for running multiple Tesseract instances more efficiently.
	WASMCache wazero.CompilationCache
}

// New creates a new Tesseract class that is ready for use.
// The Tesseract WASM is initialized with the given trainingdata, language and variable options.
// Each Tesseract object is NOT safe for concurrent use.
func New(ctx context.Context, cfg Config) (t *Tesseract, err error) {
	if cfg.TrainingData == nil {
		return nil, fmt.Errorf("Config.TrainingData is required")
	}

	t = &Tesseract{
		embindEngine: embind.CreateEngine(embind.NewConfig()),
		cfg:          cfg,
	}
	waRTCfg := wazero.NewRuntimeConfig()
	if t.cfg.WASMCache != nil {
		waRTCfg = waRTCfg.WithCompilationCache(t.cfg.WASMCache)
	}
	t.waRT = wazero.NewRuntimeWithConfig(ctx, waRTCfg)

	ctx = t.embindEngine.Attach(ctx)
	logPrefix := "gogosseract.New"

	t.module, err = wasm.CompileTesseract(ctx, t.waRT, t.embindEngine, cfg.CompileConfig)
	if err != nil {
		return nil, fmt.Errorf(logPrefix+" %w", err)
	}

	t.ocrEngine, err = gen.NewClassOCREngine(t.embindEngine, ctx)
	if err != nil {
		return nil, fmt.Errorf(logPrefix+" gen.NewClassOCREngine %w", err)
	}

	trainingDataView, err := t.createByteView(ctx, cfg.TrainingData)
	if err != nil {
		return nil, fmt.Errorf(logPrefix+" %w", err)
	}
	defer trainingDataView.Delete(ctx)

	if cfg.Language == "" {
		cfg.Language = "eng"
	}

	ocrErr, err := t.ocrEngine.LoadModel(ctx, trainingDataView, cfg.Language)
	if err != nil || ocrErr != "" {
		return nil, fmt.Errorf(logPrefix+" ocrEngine.LoadModel ocrErr (%s) %w", ocrErr, err)
	}

	if len(cfg.Variables) == 0 {
		cfg.Variables = map[string]string{
			"tessedit_pageseg_mode": "3", // tesseract::PSM_AUTO
		}
	}

	for k, v := range cfg.Variables {
		ocrErr, err := t.ocrEngine.SetVariable(ctx, k, v)
		if err != nil || ocrErr != "" {
			return nil, fmt.Errorf(logPrefix+" ocrEngine.SetVariable ocrErr (%s) %w", ocrErr, err)
		}
	}

	return t, nil
}

type Tesseract struct {
	embindEngine embind.Engine
	waRT         wazero.Runtime
	module       api.Module
	ocrEngine    *gen.ClassOCREngine
	cfg          Config
}

// LoadImage clears any previously loaded images, and loads the provided img into Tesseract WASM
// for parsing. Unfortunately the image is fully copied to memory a few times.
// Leptonica parses it into a Pix object and Tesseract copies that Pix object internally.
// Keep that in mind when working with large images.
func (t *Tesseract) LoadImage(ctx context.Context, img io.Reader) error {
	logPrefix := "Tesseract.LoadImage"

	if err := t.ocrEngine.ClearImage(ctx); err != nil {
		return fmt.Errorf(logPrefix+" t.ocrEngine.ClearImage %w", err)
	}

	imgByteView, err := t.createByteView(ctx, img)
	if err != nil {
		return fmt.Errorf(logPrefix+" createByteView %w", err)
	}
	// As Leptonica will copy the image into it's Pix object, we can free it ASAP
	defer imgByteView.Delete(ctx)

	ocrErr, err := t.ocrEngine.LoadImage(ctx, imgByteView)
	if err != nil || ocrErr != "" {
		return fmt.Errorf(logPrefix+" ocrEngine.LoadImage ocrErr=(%s) %w", ocrErr, err)
	}

	return nil
}

// GetText parses a previously loaded image for text. progressCB is called with a percentage
// for tracking Tesseract's recognition progress.
func (t *Tesseract) GetText(ctx context.Context, progressCB func(int32)) (string, error) {
	if progressCB == nil {
		progressCB = func(i int32) {}
	}
	text, err := t.ocrEngine.GetText(ctx, progressCB)
	if err != nil {
		return "", fmt.Errorf("Tesseract.GetText ocrEngine.GetText %w", err)
	}
	return text, nil
}

// GetHOCR parses a previously loaded image for HOCR text. progressCB is called with a percentage
// for tracking Tesseract's recognition progress.
func (t *Tesseract) GetHOCR(ctx context.Context, progressCB func(int32)) (string, error) {
	if progressCB == nil {
		progressCB = func(i int32) {}
	}
	text, err := t.ocrEngine.GetHOCR(ctx, progressCB)
	if err != nil {
		return "", fmt.Errorf("Tesseract.GetHOCR ocrEngine.GetHOCR %w", err)
	}
	return text, nil
}

// Close shuts down all the resources associated with the Tesseract object.
func (t *Tesseract) Close(ctx context.Context) error {
	logPrefix := "Tesseract.Close"
	if err := t.ocrEngine.ClearImage(ctx); err != nil {
		return fmt.Errorf(logPrefix+" t.ocrEngine.ClearImage %w", err)
	}
	if err := t.ocrEngine.Delete(ctx); err != nil {
		return fmt.Errorf(logPrefix+" t.ocrEngine.Delete %w", err)
	}
	return t.waRT.Close(ctx)
}

// createByteView streams an io.Reader into WASM memory using io.Copy, emscripten::typed_memory_view and minimal memory.
// Works optimally if io.Reader is an io.ReadSeeker (like an os.File) or a bytes.Buffer.
func (t *Tesseract) createByteView(ctx context.Context, reader io.Reader) (*gen.ClassByteView, error) {
	logPrefix := "Tesseract.createByteView"
	size, err := wasm.GetReaderSize(ctx, &reader)
	if err != nil {
		return nil, fmt.Errorf(logPrefix+" %w", err)
	}
	// Now that we have the size, expose WASM memory for writing into.
	byteView, err := gen.NewClassByteView(t.embindEngine, ctx, uint32(size))
	if err != nil {
		return nil, fmt.Errorf(logPrefix+" gen.NewClassByteView %w", err)
	}

	byteViewDataI, err := byteView.Data(ctx)
	if err != nil {
		return nil, fmt.Errorf(logPrefix+" gen.NewClassByteView %w", err)
	}
	byteViewData, ok := byteViewDataI.([]byte)
	if !ok {
		return nil, fmt.Errorf(logPrefix+" byteViewDataI unexpected type %T", byteViewDataI)
	}

	byteViewBuffer := bytes.NewBuffer(byteViewData)
	byteViewBuffer.Reset()

	written, err := io.Copy(byteViewBuffer, reader)
	if err != nil {
		return nil, fmt.Errorf(logPrefix+" io.Copy %w", err)
	}
	if int64(size) != written {
		return nil, fmt.Errorf(logPrefix+" io.Copy only wrote %d/%d bytes", written, size)
	}

	return byteView, nil
}
