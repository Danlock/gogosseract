package gogosseract

import (
	"context"
	"fmt"
	"io"

	"github.com/danlock/gogosseract/internal/gen"
	"github.com/danlock/gogosseract/internal/wasm"
	embind "github.com/jerbob92/wazero-emscripten-embind"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

type TesseractConfig struct {
	// Languages Tesseract scans for. Defaults to "eng".
	Language string
	// Training Data Tesseract uses. Required. Must support the provided language. https://github.com/tesseract-ocr/tessdata_fast for more details.
	TrainingData []byte
	// Variables are optionally passed into Tesseract as variable config options. http://www.sk-spell.sk.cx/tesseract-ocr-parameters-in-302-version
	Variables map[string]string
}

// NewTesseract creates a new Tesseract class that is ready for use.
// The Tesseract WASM is initialized with the given trainingdata, language and variable options.
// Each Tesseract object is NOT safe for concurrent use.
func NewTesseract(ctx context.Context, cfg TesseractConfig) (t *Tesseract, err error) {
	if len(cfg.TrainingData) == 0 {
		return nil, fmt.Errorf("TesseractConfig.TrainingData is required")
	}

	t.embindEngine = embind.CreateEngine(embind.NewConfig())
	t.waRT = wazero.NewRuntime(ctx)
	ctx = t.embindEngine.Attach(ctx)
	logPrefix := "NewTesseract"

	t.module, err = wasm.CompileTesseract(ctx, t.waRT, t.embindEngine)
	if err != nil {
		return nil, fmt.Errorf(logPrefix+" wasm.CompileTesseract %w", err)
	}

	t.ocrEngine, err = gen.NewClassOCREngine(t.embindEngine, ctx)
	if err != nil {
		return nil, fmt.Errorf(logPrefix+" gen.NewClassOCREngine %w", err)
	}

	trainPtr, err := wasm.WriteBytes(ctx, t.module, cfg.TrainingData)
	if err != nil {
		return nil, fmt.Errorf(logPrefix+" WriteBytes %w", err)
	}
	defer wasm.Free(ctx, t.module, trainPtr)

	if cfg.Language == "" {
		cfg.Language = "eng"
	}

	ocrErr, err := t.ocrEngine.LoadModel(ctx, uint32(trainPtr), uint32(len(cfg.TrainingData)), cfg.Language)
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
			return nil, fmt.Errorf(logPrefix+" ocrEngine.LoadModel ocrErr (%s) %w", ocrErr, err)
		}
	}

	return t, nil
}

type Tesseract struct {
	embindEngine embind.Engine
	waRT         wazero.Runtime
	module       api.Module
	ocrEngine    *gen.ClassOCREngine
}

// LoadImage for convenience loads from an io.Reader, for reading from a file for example.
func (t *Tesseract) LoadImage(ctx context.Context, imgSrc io.Reader) error {
	logPrefix := "Tesseract.LoadImage"
	img, err := io.ReadAll(imgSrc)
	if err != nil {
		return fmt.Errorf(logPrefix+" io.ReadAll %w", err)
	}
	return t.LoadImageBuf(ctx, img)
}

// LoadImageBuf clears any previously loaded images, and loads the provided imgSrc into Tesseract WASM
// for parsing. Unfortunately the image is copied several times. Writing into WASM memory,
// being parsed by Leptonica into it's Pix object and when Tesseract copies it to it's internal Pix object.
// Keep that in mind when working with large images.
func (t *Tesseract) LoadImageBuf(ctx context.Context, img []byte) error {
	logPrefix := "Tesseract.LoadImageBuf"

	if err := t.ocrEngine.ClearImage(ctx); err != nil {
		return fmt.Errorf(logPrefix+" t.ocrEngine.ClearImage %w", err)
	}

	imgPtr, err := wasm.WriteBytes(ctx, t.module, img)
	if err != nil {
		return fmt.Errorf(logPrefix+" wasm.WriteBytes %w", err)
	}
	// As Leptonica will copy the image into it's Pix object, we can free it ASAP
	defer wasm.Free(ctx, t.module, imgPtr)

	results, err := t.module.ExportedFunction("pixReadMem").Call(ctx, imgPtr, uint64(len(img)))
	if err != nil {
		return fmt.Errorf(logPrefix+" pixReadMem results=(%v) %w", results, err)
	}
	if len(results) != 1 || results[0] == 0 {
		return fmt.Errorf(logPrefix + " pixReadMem returned nullptr")
	}
	pixPtr := results[0]

	ocrErr, err := t.ocrEngine.LoadImage(ctx, uint32(pixPtr))
	if err != nil || ocrErr != "" {
		return fmt.Errorf(logPrefix+" ocrEngine.LoadImage ocrErr=(%v) %w", ocrErr, err)
	}

	return nil
}

// GetText parses a previously loaded image for text. progressCB is called with a percentage
// for tracking Tesseract's recognition progress.
func (t *Tesseract) GetText(ctx context.Context, progressCB func(int)) (string, error) {
	text, err := t.ocrEngine.GetText(ctx, progressCB)
	if err != nil {
		return "", fmt.Errorf("Tesseract.GetText ocrEngine.GetText %w", err)
	}
	return text, nil
}

// GetHOCR parses a previously loaded image for HOCR text. progressCB is called with a percentage
// for tracking Tesseract's recognition progress.
func (t *Tesseract) GetHOCR(ctx context.Context, progressCB func(int)) (string, error) {
	text, err := t.ocrEngine.GetHOCR(ctx, progressCB)
	if err != nil {
		return "", fmt.Errorf("Tesseract.GetHOCR ocrEngine.GetHOCR %w", err)
	}
	return text, nil
}

// Close shuts down all the resources associated with the Tesseract class.
func (t *Tesseract) Close(ctx context.Context) error {
	return t.waRT.Close(ctx)
}
