package gogosseract_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/danlock/gogosseract"
	"github.com/danlock/pkg/test"
	"github.com/google/go-cmp/cmp"
)

func BenchmarkGogosseract(b *testing.B) {
	ctx := context.Background()
	tess, err := gogosseract.New(ctx, gogosseract.Config{
		TrainingData: bytes.NewBuffer(engTrainedData),
	})
	test.FailOnError(b, err)
	defer func() {
		test.FailOnError(b, tess.Close(ctx))
	}()

	tests := []struct {
		name     string
		imgSrc   []byte
		opts     gogosseract.LoadImageOptions
		isHOCR   bool
		wantText string
	}{
		{
			"logo text",
			logoImg,
			gogosseract.LoadImageOptions{},
			false,
			logoText,
		},
		{
			"docs text",
			docsImg,
			gogosseract.LoadImageOptions{},
			false,
			docsText,
		},
		{
			"underline removal",
			underlineImg,
			gogosseract.LoadImageOptions{
				RemoveUnderlines: true,
			},
			false,
			underlineText,
		},
	}
	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				test.FailOnError(b, tess.LoadImage(ctx, bytes.NewBuffer(tt.imgSrc), tt.opts))
				var text string
				if tt.isHOCR {
					text, err = tess.GetHOCR(ctx, nil)
				} else {
					text, err = tess.GetText(ctx, nil)
				}
				if text != tt.wantText {
					b.Fatalf(cmp.Diff(text, tt.wantText))
				}
			}
		})
	}
}
