package gogosseract

import (
	"bytes"
	"context"
	_ "embed"
	"io"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tetratelabs/wazero"
)

//go:embed internal/wasm/testdata/eng.traineddata
var engTrainedData []byte

//go:embed internal/wasm/testdata/docs.png
var docsImg []byte

//go:embed internal/wasm/testdata/logo.png
var logoImg []byte

func TestNewTesseract(t *testing.T) {
	ctx := context.Background()
	sharedWART := wazero.NewRuntime(ctx)
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
				WazeroRT:     sharedWART,
			},
			false,
		},
		{
			"success with lang",
			TesseractConfig{
				TrainingData: bytes.NewBuffer(engTrainedData),
				Language:     "tesseractishappywithwhateveraslongasyourtrainingdatamatchesup",
				WazeroRT:     sharedWART,
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
			"o\n\nkubenav\n",
		},
		{
			"logo file",
			logoFile,
			false,
			"o\n\nkubenav\n",
		},
		{
			"logo with forced copy",
			JustAReader{bytes.NewBuffer(logoImg)},
			false,
			"o\n\nkubenav\n",
		},
		{
			"docs",
			bytes.NewBuffer(docsImg),
			false,
			"Request body\n\nParameter Required Type\ngeoname false integer\ncredits false integer\ncellTowers false array\nwifiAccessPoints false array\nbluetoothBeacons false array\nsensors false array\n\nfallbacks false array\n",
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
			text, err := tess.GetText(ctx, nil)
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
