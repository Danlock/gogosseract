package gogosseract

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"

	"github.com/danlock/gogosseract/internal/wasm"
	"github.com/tetratelabs/wazero/api"
)

type TesseractConfig struct {
	// Training Data Tesseract uses. Required. Must support the provided language. https://github.com/tesseract-ocr/tessdata_fast for more details.
	TrainingData io.Reader
	// Languages Tesseract scans for. Defaults to "eng".
	Language string
	// Variables are optionally passed into Tesseract as variable config options. Some options are listed at http://www.sk-spell.sk.cx/tesseract-ocr-parameters-in-302-version
	Variables map[string]string
	// WASM is an optional Webassembly runtime and compilation object. Create with NewWASM.
	// While each Tesseract is not safe for conccurrent use, we can just create several instead.
	// Sharing the WASMModule allows us to do this efficiently, reusing the compiled WASM and Webassembly runtime.
	// Note that Tesseract.Close will not Close the WASMModule if passed in, instead it is your responsibility.
	WASM *WASMModule
}

// NewTesseract creates a new Tesseract class that is ready for use.
// The Tesseract WASM is initialized with the given trainingdata, language and variable options.
// Each Tesseract object is NOT safe for concurrent use.
func NewTesseract(ctx context.Context, cfg TesseractConfig) (t *Tesseract, err error) {
	logPrefix := "NewTesseract"

	if cfg.TrainingData == nil {
		return nil, fmt.Errorf(logPrefix + " requires TesseractConfig.TrainingData")
	}
	if cfg.Language == "" {
		cfg.Language = "eng"
	}
	if len(cfg.Variables) == 0 {
		cfg.Variables = map[string]string{
			"tessedit_pageseg_mode": "3", // tesseract::PSM_AUTO
		}
	}
	t = &Tesseract{
		cfg:        cfg,
		WASMModule: cfg.WASM,
	}
	if cfg.WASM == nil {
		t.WASMModule, err = NewWASMModule(ctx, WASMConfig{})
		if err != nil {
			return nil, fmt.Errorf(logPrefix+" %w", err)
		}
	}

	// Create an OCREngine, our WASM wrapper of the tesseract::TessBaseAPI class
	// Then stream the training model data to it, initializing it for image parsing.
	results, err := t.module.ExportedFunction("CreateOCREngine").Call(ctx)
	if err != nil || len(results) != 1 || results[0] == 0 {
		return nil, fmt.Errorf(logPrefix+" CreateOCREngine results=(%v) %w", results, err)
	}
	t.ocrEnginePtr = results[0]

	trainingDataPtr, trainingDataSize, err := t.writeReaderWithLength(ctx, cfg.TrainingData)
	if err != nil {
		return nil, fmt.Errorf(logPrefix+" %w", err)
	}
	defer wasm.Free(ctx, t.module, trainingDataPtr)

	langPtr, err := wasm.WriteString(ctx, t.module, cfg.Language)
	if err != nil {
		return nil, fmt.Errorf(logPrefix+" %w", err)
	}
	defer wasm.Free(ctx, t.module, langPtr)

	results, err = t.module.ExportedFunction("InitOCREngine").Call(ctx, t.ocrEnginePtr, trainingDataPtr, trainingDataSize, langPtr)
	if err != nil || len(results) != 1 || api.DecodeI32(results[0]) == -1 {
		return nil, fmt.Errorf(logPrefix+" InitOCREngine failed initializing training data %w", err)
	}

	// for k, v := range cfg.Variables {
	// 	ocrErr, err := t.ocrEngine.SetVariable(ctx, k, v)
	// 	if err != nil || ocrErr != "" {
	// 		return nil, fmt.Errorf(logPrefix+" ocrEngine.SetVariable ocrErr (%s) %w", ocrErr, err)
	// 	}
	// }

	return t, nil
}

type Tesseract struct {
	*WASMModule
	cfg          TesseractConfig
	ocrEnginePtr uint64
}

// LoadImage clears any previously loaded images, and loads the provided img into Tesseract WASM
// for parsing. Unfortunately the image is fully copied to memory a few times.
// Leptonica parses it into a Pix object and Tesseract copies that Pix object internally.
// Keep that in mind when working with large images.
func (t *Tesseract) LoadImage(ctx context.Context, img io.Reader) error {
	logPrefix := "Tesseract.LoadImage"

	// if err := t.ocrEngine.ClearImage(ctx); err != nil {
	// 	return fmt.Errorf(logPrefix+" t.ocrEngine.ClearImage %w", err)
	// }

	imgPtr, imgSize, err := t.writeReaderWithLength(ctx, img)
	if err != nil {
		return fmt.Errorf(logPrefix+" %w", err)
	}
	// As Leptonica will copy the image into it's Pix object, we can free it ASAP
	defer wasm.Free(ctx, t.module, imgPtr)

	results, err := t.module.ExportedFunction("LoadImageIntoOCREngine").Call(ctx, t.ocrEnginePtr, imgPtr, imgSize)
	if err != nil || len(results) != 1 || api.DecodeI32(results[0]) == -1 {
		return fmt.Errorf(logPrefix+" LoadImageIntoOCREngine failed %w", err)
	}

	return nil
}

// GetText parses a previously loaded image for text. progressCB is called with a percentage
// for tracking Tesseract's recognition progress.
func (t *Tesseract) GetText(ctx context.Context) (string, error) {
	logPrefix := "Tesseract.GetText"

	results, err := t.module.ExportedFunction("GetTextFromOCREngine").Call(ctx, t.ocrEnginePtr, 0)
	if err != nil || len(results) != 1 || results[0] == 0 {
		return "", fmt.Errorf(logPrefix+" GetTextFromOCREngine failed %w", err)
	}

	return wasm.ReadAndFreeString(ctx, t.module, results[0]), nil
}

// GetHOCR parses a previously loaded image for HOCR text. progressCB is called with a percentage
// for tracking Tesseract's recognition progress.
// func (t *Tesseract) GetHOCR(ctx context.Context, progressCB func(int32)) (string, error) {
// 	if progressCB == nil {
// 		progressCB = func(int32) {}
// 	}
// 	text, err := t.ocrEngine.GetHOCR(ctx, progressCB)
// 	if err != nil {
// 		return "", fmt.Errorf("Tesseract.GetHOCR ocrEngine.GetHOCR %w", err)
// 	}
// 	return text, nil
// }

// Close shuts down all the resources associated with the Tesseract object
// func (t *Tesseract) Close(ctx context.Context) error {
// 	logPrefix := "Tesseract.Close"
// 	if err := t.ocrEngine.ClearImage(ctx); err != nil {
// 		return fmt.Errorf(logPrefix+" t.ocrEngine.ClearImage %w", err)
// 	}
// 	if err := t.ocrEngine.Delete(ctx); err != nil {
// 		return fmt.Errorf(logPrefix+" t.ocrEngine.Delete %w", err)
// 	}
// 	// Only close the WASMModule if we made it ourselves
// 	if t.cfg.WASM != nil {
// 		return nil
// 	}
// 	return t.WASMModule.Close(ctx)
// }

// readerWithLen is an interface that generalizes code for bytes.Buffer to anything with a Len() method.
type readerWithLen interface {
	io.Reader
	Len() int
}

var _ = readerWithLen(new(bytes.Buffer))

// writeReaderWithLength streams an io.Reader into WASM memory using io.Copy, emscripten::typed_memory_view and minimal memory.
// Works optimally if io.Reader is an io.ReadSeeker (like os.File) or a readerWithLen (like bytes.Buffer).
func (t *Tesseract) writeReaderWithLength(ctx context.Context, reader io.Reader) (uint64, uint64, error) {
	logPrefix := "Tesseract.writeReaderWithLength"
	var size uint64
	switch src := reader.(type) {
	case readerWithLen:
		size = uint64(src.Len())
	case io.Seeker:
		// This case covers os.File while being friendly to any other io.ReadSeeker's
		streamLen, err := src.Seek(0, io.SeekEnd)
		if err != nil {
			return 0, 0, fmt.Errorf(logPrefix+" io.Seeker.Seek SeekEnd %w", err)
		}
		// Seek back to the beginning so we can read it all
		if _, err := src.Seek(0, io.SeekStart); err != nil {
			return 0, 0, fmt.Errorf(logPrefix+" io.Seeker.Seek SeekStart %w", err)
		}
		size = uint64(streamLen)
	case nil:
		return 0, 0, fmt.Errorf(logPrefix + " got nil io.Reader")
	default:
		// We have failed to avoid copying the data into memory to get the size...
		// git commit sudoku
		buf := new(bytes.Buffer)
		bufLen, err := io.Copy(buf, reader)
		if err != nil {
			return 0, 0, fmt.Errorf(logPrefix+" io.Copy %w", err)
		}
		// since we read reader, it's empty. point it at buf now.
		reader = buf
		size = uint64(bufLen)
	}

	if size > math.MaxUint32 {
		return 0, 0, fmt.Errorf(logPrefix+" io.Reader size (%d) too large", size)
	} else if size == 0 {
		return 0, 0, fmt.Errorf(logPrefix + " io.Reader was empty")
	}
	addr, err := wasm.WriteFromReader(ctx, t.module, reader, uint32(size))
	if err != nil {
		return 0, 0, fmt.Errorf(logPrefix+" %w", err)
	}
	return addr, size, nil
}
