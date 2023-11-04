package gogosseract

import (
	"bytes"
	"context"
	"io"

	"github.com/danlock/gogosseract/internal/gen"
	"github.com/danlock/gogosseract/internal/wasm"
	"github.com/danlock/pkg/errors"
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
		return nil, errors.Errorf("Config.TrainingData is required")
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

	t.module, err = wasm.CompileTesseract(ctx, t.waRT, t.embindEngine, cfg.CompileConfig)
	if err != nil {
		return nil, errors.Errorf("%w", err)
	}

	t.ocrEngine, err = gen.NewClassOCREngine(t.embindEngine, ctx)
	if err != nil {
		return nil, errors.Errorf("gen.NewClassOCREngine %w", err)
	}

	trainingDataView, err := t.createByteView(ctx, cfg.TrainingData)
	if err != nil {
		return nil, errors.Errorf("%w", err)
	}
	defer trainingDataView.Delete(ctx)

	if cfg.Language == "" {
		cfg.Language = "eng"
	}

	ocrErr, err := t.ocrEngine.LoadModel(ctx, trainingDataView, cfg.Language)
	if err != nil || ocrErr != "" {
		return nil, errors.Errorf("ocrEngine.LoadModel ocrErr (%s) %w", ocrErr, err)
	}

	if len(cfg.Variables) == 0 {
		cfg.Variables = map[string]string{
			"tessedit_pageseg_mode": "3", // tesseract::PSM_AUTO
		}
	}

	for k, v := range cfg.Variables {
		ocrErr, err := t.ocrEngine.SetVariable(ctx, k, v)
		if err != nil || ocrErr != "" {
			return nil, errors.Errorf("ocrEngine.SetVariable ocrErr (%s) %w", ocrErr, err)
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

type LoadImageOptions struct {
	// RemoveUnderlines uses Leptonica (C img lib) to remove the underlines from the given image. Copies a lot.
	RemoveUnderlines bool
}

// LoadImage clears any previously loaded images, and loads the provided img into Tesseract WASM
// for parsing. Unfortunately the image is fully copied to memory a few times.
// Leptonica parses it into a Pix object and Tesseract copies that Pix object internally.
// Keep that in mind when working with large images.
func (t *Tesseract) LoadImage(ctx context.Context, img io.Reader, opts LoadImageOptions) error {
	if err := t.ClearImage(ctx); err != nil {
		return errors.Wrap(err)
	}

	imgByteView, err := t.createByteView(ctx, img)
	if err != nil {
		return errors.Wrap(err)
	}
	// As Leptonica will copy the image into it's Pix object, we can free it ASAP
	defer imgByteView.Delete(ctx)

	ocrErr, err := t.ocrEngine.LoadImage(ctx, imgByteView, opts.RemoveUnderlines)
	if err != nil || ocrErr != "" {
		return errors.Errorf("ocrEngine.LoadImage ocrErr=(%s) %w", ocrErr, err)
	}

	return nil
}

// ClearImage clears the image from within Tesseract. LoadImage calls this for you.
func (t *Tesseract) ClearImage(ctx context.Context) error {
	if err := t.ocrEngine.ClearImage(ctx); err != nil {
		return errors.Errorf("ocrEngine.ClearImage %w", err)
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
		return "", errors.Errorf("ocrEngine.GetText %w", err)
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
		return "", errors.Errorf("ocrEngine.GetHOCR %w", err)
	}
	return text, nil
}

// Close shuts down all the resources associated with the Tesseract object.
func (t *Tesseract) Close(ctx context.Context) error {
	if err := t.ClearImage(ctx); err != nil {
		return errors.Wrap(err)
	}

	if err := t.ocrEngine.Delete(ctx); err != nil {
		return errors.Errorf(" ocrEngine.Delete %w", err)
	}
	return t.waRT.Close(ctx)
}

// createByteView streams an io.Reader into WASM memory using io.Copy, emscripten::typed_memory_view and minimal memory.
// Works optimally if io.Reader is an io.ReadSeeker (like an os.File) or a bytes.Buffer.
func (t *Tesseract) createByteView(ctx context.Context, reader io.Reader) (*gen.ClassByteView, error) {
	size, err := wasm.GetReaderSize(ctx, &reader)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	// Now that we have the size, expose WASM memory for writing into.
	byteView, err := gen.NewClassByteView(t.embindEngine, ctx, uint32(size))
	if err != nil {
		return nil, errors.Errorf("gen.NewClassByteView malloc %w", err)
	}

	byteViewDataI, err := byteView.Data(ctx)
	if err != nil {
		return nil, errors.Errorf("gen.NewClassByteView %w", err)
	}
	byteViewData, ok := byteViewDataI.([]byte)
	if !ok {
		return nil, errors.Errorf("byteViewDataI unexpected type %T", byteViewDataI)
	}

	byteViewBuffer := bytes.NewBuffer(byteViewData)
	byteViewBuffer.Reset()

	written, err := io.Copy(byteViewBuffer, reader)
	if err != nil {
		return nil, errors.Errorf("io.Copy %w", err)
	}
	if int64(size) != written {
		return nil, errors.Errorf("io.Copy only wrote %d/%d bytes", written, size)
	}

	return byteView, nil
}
