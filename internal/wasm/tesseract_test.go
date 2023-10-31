package wasm

import (
	"context"
	_ "embed"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	embind "github.com/jerbob92/wazero-emscripten-embind"
	"github.com/tetratelabs/wazero"
)

func TestCompileTesseract(t *testing.T) {
	embEng := embind.CreateEngine(embind.NewConfig())
	ctx := embEng.Attach(context.Background())
	waRT := wazero.NewRuntime(ctx)
	defer waRT.Close(ctx)

	_, err := CompileTesseract(ctx, waRT, embEng)
	if err != nil {
		t.Fatalf("CompileTesseract() error = %v", err)
	}
}

//go:embed testdata/eng.traineddata
var tesseractEngTrainedData []byte

//go:embed testdata/docs.png
var testImg []byte

func TestParseTextFromImage(t *testing.T) {
	embEng := embind.CreateEngine(embind.NewConfig())
	ctx := embEng.Attach(context.Background())
	waRT := wazero.NewRuntime(ctx)
	defer waRT.Close(ctx)

	tessMod, err := CompileTesseract(ctx, waRT, embEng)
	if err != nil {
		t.Fatalf("CompileTesseract() error = %v", err)
	}

	wanted := `Request body
Parameter Required Type
geoname false integer
credits false integer
cellTowers false array
wifiAccessPoints false array
bluetoothBeacons false array
sensors false array
fallbacks false array
	`

	testStr, err := parseTextFromImage(ctx, tessMod, embEng, testImg, tesseractEngTrainedData)
	diff := cmp.Diff(strings.TrimSpace(testStr), strings.TrimSpace(wanted))
	if err != nil || diff != "" {
		t.Log(testStr)
		t.Fatalf("ParseTextFromImage() diff = %s error = %v", diff, err)
	}
}
