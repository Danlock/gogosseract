package gogosseract

import (
	"bytes"
	"context"
	_ "embed"
	"io"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/sync/errgroup"
)

//go:embed internal/wasm/testdata/eng.traineddata
var engTrainedData []byte

//go:embed internal/wasm/testdata/docs.png
var docsImg []byte

const docsText = "Request body\n\nParameter Required Type\ngeoname false integer\ncredits false integer\ncellTowers false array\nwifiAccessPoints false array\nbluetoothBeacons false array\nsensors false array\n\nfallbacks false array\n"

//go:embed internal/wasm/testdata/logo.png
var logoImg []byte

const logoText = "o\n\nkubenav\n"

func TestNewTesseract(t *testing.T) {
	ctx := context.Background()
	sharedWASM, err := NewWASMModule(ctx, WASMConfig{})
	if err != nil {
		t.Fatalf("NewWASM %v", err)
	}

	tests := []struct {
		name    string
		cfg     TesseractConfig
		wantErr bool
	}{
		{
			"no training data",
			TesseractConfig{},
			true,
		},
		{
			"empty training data",
			TesseractConfig{TrainingData: bytes.NewBuffer([]byte{})},
			true,
		},
		{
			"aggressively nil training data",
			TesseractConfig{TrainingData: io.Reader(nil)},
			true,
		},
		{
			"bad variables",
			TesseractConfig{
				TrainingData: bytes.NewBuffer(engTrainedData),
				Variables:    map[string]string{"asdf": "qwer"},
			},
			true,
		},
		{
			"success no lang",
			TesseractConfig{
				TrainingData: bytes.NewBuffer(engTrainedData),
				WASM:         sharedWASM,
			},
			false,
		},
		{
			"success with lang",
			TesseractConfig{
				TrainingData: bytes.NewBuffer(engTrainedData),
				Language:     "tesseractishappywithwhateveraslongasyourtrainingdatamatchesup",
				WASM:         sharedWASM,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTesseract(ctx, tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTesseract() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

type JustAReader struct {
	buf *bytes.Buffer
}

func (j JustAReader) Read(p []byte) (n int, err error) {
	return j.buf.Read(p)
}

func TestTesseract_GetText(t *testing.T) {
	ctx := context.Background()
	tess, err := NewTesseract(ctx, TesseractConfig{TrainingData: bytes.NewBuffer(engTrainedData)})
	if err != nil {
		t.Fatalf("NewTesseract %v", err)
	}
	logoFile, err := os.Open("./internal/wasm/testdata/logo.png")
	if err != nil {
		t.Fatalf("os.Open %v", err)
	}

	tests := []struct {
		name     string
		imgSrc   io.Reader
		wantErr  bool
		wantText string
	}{
		{
			"nada",
			&bytes.Buffer{},
			true,
			"",
		},
		{
			"logo",
			bytes.NewBuffer(logoImg),
			false,
			logoText,
		},
		{
			"logo file",
			logoFile,
			false,
			logoText,
		},
		{
			"logo with forced copy",
			JustAReader{bytes.NewBuffer(logoImg)},
			false,
			logoText,
		},
		{
			"docs",
			bytes.NewBuffer(docsImg),
			false,
			docsText,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tess.LoadImage(ctx, tt.imgSrc); (err != nil) != tt.wantErr {
				t.Fatalf("Tesseract.LoadImage() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			text, err := tess.GetText(ctx)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Tesseract.GetText() error = %v, wantErr %v", err, tt.wantErr)
			}
			diff := cmp.Diff(text, tt.wantText)
			if diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}

func TestTesseract_GetText_Concurrently(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	waMo, err := NewWASMModule(ctx, WASMConfig{})
	if err != nil {
		t.Fatalf("NewWASM %v", err)
	}
	tessPool := make([]*Tesseract, 5)
	var group errgroup.Group
	results := make(chan string, len(tessPool))
	for i := range tessPool {
		tessPool[i], err = NewTesseract(ctx, TesseractConfig{
			TrainingData: bytes.NewBuffer(engTrainedData),
			WASM:         waMo,
		})
		if err != nil {
			t.Fatalf("i=%d %v", i, err)
		}
		i := i
		group.Go(func() error {
			if err := tessPool[i].LoadImage(ctx, bytes.NewBuffer(docsImg)); err != nil {
				return err
			}
			result, err := tessPool[i].GetText(ctx)
			if err != nil {
				return err
			}
			results <- result
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		t.Fatalf("Parallel Tesseracts client failed %v", err)
	}
	for range tessPool {
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for results")
		case r := <-results:
			if r != docsText {
				t.Fatalf("results incorrect (%s)", cmp.Diff(r, docsText))
			}
		}
	}
}
