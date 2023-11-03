package gogosseract_test

import (
	"bytes"
	"context"
	_ "embed"
	"io"
	"os"
	"testing"
	"time"

	"github.com/danlock/gogosseract"
	"github.com/google/go-cmp/cmp"
	"github.com/tetratelabs/wazero"
	"golang.org/x/sync/errgroup"
)

//go:embed internal/wasm/testdata/eng.traineddata
var engTrainedData []byte

//go:embed internal/wasm/testdata/docs.png
var docsImg []byte

const docsText = "Request body\n\nParameter Required Type\ngeoname false integer\ncredits false integer\ncellTowers false array\nwifiAccessPoints false array\nbluetoothBeacons false array\nsensors false array\n\nfallbacks false array\n"
const docsHOCR = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="en" lang="en">
<head>
  <title>hOCR text</title>
  <meta http-equiv="Content-Type" content="text/html;charset=utf-8"/>
  <meta name='ocr-system' content='tesseract 5.3.0' />
  <meta name='ocr-capabilities' content='ocr_page ocr_carea ocr_par ocr_line ocrx_word ocrp_wconf' />
</head>
<body>
    <div class='ocr_page' id='page_1' title='image "unknown"; bbox 0 0 285 678; ppageno 0; scan_res 96 96'>
   <div class='ocr_carea' id='block_1_1' title="bbox 4 1 128 18">
    <p class='ocr_par' id='par_1_1' lang='eng' title="bbox 4 1 128 18">
     <span class='ocr_line' id='line_1_1' title="bbox 4 1 128 18; baseline 0 -4; x_size 17; x_descenders 4; x_ascenders 3">
      <span class='ocrx_word' id='word_1_1' title='bbox 4 1 78 18; x_wconf 95'>Request</span>
      <span class='ocrx_word' id='word_1_2' title='bbox 84 1 128 18; x_wconf 95'>body</span>
     </span>
    </p>
   </div>
   <div class='ocr_carea' id='block_1_2' title="bbox 13 44 269 540">
    <p class='ocr_par' id='par_1_2' lang='eng' title="bbox 13 44 269 540">
     <span class='ocr_line' id='line_1_2' title="bbox 14 44 257 70; baseline 0 -7; x_size 18; x_descenders 3; x_ascenders 5">
      <span class='ocrx_word' id='word_1_3' title='bbox 14 44 81 70; x_wconf 95'>Parameter</span>
      <span class='ocrx_word' id='word_1_4' title='bbox 146 52 205 66; x_wconf 93'>Required</span>
      <span class='ocrx_word' id='word_1_5' title='bbox 226 53 257 66; x_wconf 70'>Type</span>
     </span>
     <span class='ocr_line' id='line_1_3' title="bbox 14 91 269 105; baseline 0 -3; x_size 22.592106; x_descenders 5.5; x_ascenders 5.6973686">
      <span class='ocrx_word' id='word_1_6' title='bbox 14 94 71 105; x_wconf 78'>geoname</span>
      <span class='ocrx_word' id='word_1_7' title='bbox 145 91 174 102; x_wconf 95'>false</span>
      <span class='ocrx_word' id='word_1_8' title='bbox 227 92 269 105; x_wconf 92'>integer</span>
     </span>
     <span class='ocr_line' id='line_1_4' title="bbox 14 190 269 221; baseline 0 -10; x_size 22.592106; x_descenders 5.5; x_ascenders 5.6973686">
      <span class='ocrx_word' id='word_1_9' title='bbox 14 190 54 221; x_wconf 86'>credits</span>
      <span class='ocrx_word' id='word_1_10' title='bbox 145 200 174 211; x_wconf 95'>false</span>
      <span class='ocrx_word' id='word_1_11' title='bbox 227 201 269 214; x_wconf 92'>integer</span>
     </span>
     <span class='ocr_line' id='line_1_5' title="bbox 14 310 258 324; baseline 0 -3; x_size 22.592106; x_descenders 5.5; x_ascenders 5.6973686">
      <span class='ocrx_word' id='word_1_12' title='bbox 14 310 78 321; x_wconf 84'>cellTowers</span>
      <span class='ocrx_word' id='word_1_13' title='bbox 145 310 174 321; x_wconf 95'>false</span>
      <span class='ocrx_word' id='word_1_14' title='bbox 226 313 258 324; x_wconf 49'>array</span>
     </span>
     <span class='ocr_line' id='line_1_6' title="bbox 13 374 258 388; baseline 0 -3; x_size 22.592106; x_descenders 5.5; x_ascenders 5.6973686">
      <span class='ocrx_word' id='word_1_15' title='bbox 13 374 117 385; x_wconf 76'>wifiAccessPoints</span>
      <span class='ocrx_word' id='word_1_16' title='bbox 145 374 174 385; x_wconf 72'>false</span>
      <span class='ocrx_word' id='word_1_17' title='bbox 226 377 258 388; x_wconf 51'>array</span>
     </span>
     <span class='ocr_line' id='line_1_7' title="bbox 14 462 258 476; baseline 0 -3; x_size 22.592106; x_descenders 5.5; x_ascenders 5.6973686">
      <span class='ocrx_word' id='word_1_18' title='bbox 14 462 125 473; x_wconf 80'>bluetoothBeacons</span>
      <span class='ocrx_word' id='word_1_19' title='bbox 145 462 174 473; x_wconf 63'>false</span>
      <span class='ocrx_word' id='word_1_20' title='bbox 226 465 258 476; x_wconf 49'>array</span>
     </span>
     <span class='ocr_line' id='line_1_8' title="bbox 13 526 258 540; baseline 0 -3; x_size 22.592106; x_descenders 5.5; x_ascenders 5.6973686">
      <span class='ocrx_word' id='word_1_21' title='bbox 13 529 62 537; x_wconf 92'>sensors</span>
      <span class='ocrx_word' id='word_1_22' title='bbox 145 526 174 537; x_wconf 95'>false</span>
      <span class='ocrx_word' id='word_1_23' title='bbox 226 529 258 540; x_wconf 51'>array</span>
     </span>
    </p>
   </div>
   <div class='ocr_carea' id='block_1_3' title="bbox 13 583 258 611">
    <p class='ocr_par' id='par_1_3' lang='eng' title="bbox 13 583 258 611">
     <span class='ocr_line' id='line_1_9' title="bbox 13 583 258 611; baseline 0 -9; x_size 20; x_descenders 5; x_ascenders 5">
      <span class='ocrx_word' id='word_1_24' title='bbox 13 583 67 611; x_wconf 88'>fallbacks</span>
      <span class='ocrx_word' id='word_1_25' title='bbox 145 591 174 602; x_wconf 95'>false</span>
      <span class='ocrx_word' id='word_1_26' title='bbox 226 594 258 605; x_wconf 40'>array</span>
     </span>
    </p>
   </div>
  </div>

</body>
</html>`

//go:embed internal/wasm/testdata/logo.png
var logoImg []byte

const logoText = "o\n\nkubenav\n"
const logoHOCR = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="en" lang="en">
<head>
  <title>hOCR text</title>
  <meta http-equiv="Content-Type" content="text/html;charset=utf-8"/>
  <meta name='ocr-system' content='tesseract 5.3.0' />
  <meta name='ocr-capabilities' content='ocr_page ocr_carea ocr_par ocr_line ocrx_word ocrp_wconf' />
</head>
<body>
    <div class='ocr_page' id='page_1' title='image "unknown"; bbox 0 0 512 512; ppageno 0; scan_res 72 72'>
   <div class='ocr_photo' id='block_1_1' title="bbox 72 14 440 382"></div>
   <div class='ocr_carea' id='block_1_2' title="bbox 77 19 435 378">
    <p class='ocr_par' id='par_1_1' lang='eng' title="bbox 77 19 435 378">
     <span class='ocr_caption' id='line_1_1' title="bbox 77 19 435 378; baseline 0 0; x_size 480; x_descenders 120; x_ascenders 120">
      <span class='ocrx_word' id='word_1_1' title='bbox 77 19 435 378; x_wconf 0'>o</span>
     </span>
    </p>
   </div>
   <div class='ocr_carea' id='block_1_3' title="bbox 22 399 490 490">
    <p class='ocr_par' id='par_1_2' lang='eng' title="bbox 22 399 490 490">
     <span class='ocr_caption' id='line_1_2' title="bbox 22 399 490 490; baseline -0.002 0; x_size 105.39416; x_descenders 14.39416; x_ascenders 23">
      <span class='ocrx_word' id='word_1_2' title='bbox 22 399 490 490; x_wconf 89'>kubenav</span>
     </span>
    </p>
   </div>
  </div>

</body>
</html>`

func TestNewTesseract(t *testing.T) {
	ctx := context.Background()
	cache := wazero.NewCompilationCache()
	tests := []struct {
		name    string
		cfg     gogosseract.Config
		wantErr bool
	}{
		{
			"no training data",
			gogosseract.Config{},
			true,
		},
		{
			"empty training data",
			gogosseract.Config{TrainingData: bytes.NewBuffer([]byte{})},
			true,
		},
		{
			"bad variables",
			gogosseract.Config{
				TrainingData: bytes.NewBuffer(engTrainedData),
				Variables:    map[string]string{"asdf": "qwer"},
			},
			true,
		},
		{
			"success no lang",
			gogosseract.Config{
				TrainingData: bytes.NewBuffer(engTrainedData),
				WASMCache:    cache,
			},
			false,
		},
		{
			"success with lang",
			gogosseract.Config{
				TrainingData: bytes.NewBuffer(engTrainedData),
				Language:     "tesseractishappywithwhateveraslongasyourtrainingdatamatchesup",
				WASMCache:    cache,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := gogosseract.New(ctx, tt.cfg)
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
	tess, err := gogosseract.New(ctx, gogosseract.Config{TrainingData: bytes.NewBuffer(engTrainedData)})
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

func TestTesseract_GetHOCR(t *testing.T) {
	ctx := context.Background()
	tess, err := gogosseract.New(ctx, gogosseract.Config{TrainingData: bytes.NewBuffer(engTrainedData)})
	if err != nil {
		t.Fatalf("NewTesseract %v", err)
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
			logoHOCR,
		},
		{
			"docs",
			bytes.NewBuffer(docsImg),
			false,
			docsHOCR,
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

			text, err := tess.GetHOCR(ctx, nil)
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
	cache := wazero.NewCompilationCache()

	tessPool := make([]*gogosseract.Tesseract, 5)
	var group errgroup.Group
	results := make(chan string, len(tessPool))
	var err error
	for i := range tessPool {
		tessPool[i], err = gogosseract.New(ctx, gogosseract.Config{
			TrainingData: bytes.NewBuffer(engTrainedData),
			WASMCache:    cache,
		})
		if err != nil {
			t.Fatalf("i=%d %v", i, err)
		}
		i := i
		group.Go(func() error {
			if err := tessPool[i].LoadImage(ctx, bytes.NewBuffer(docsImg)); err != nil {
				return err
			}
			result, err := tessPool[i].GetText(ctx, nil)
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
	for _, tess := range tessPool {
		if err := tess.Close(ctx); err != nil {
			t.Fatalf("Tesseract.Close err %v", err)
		}
	}
}
