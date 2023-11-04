package gogosseract_test

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/danlock/gogosseract"
	"github.com/danlock/pkg/test"
)

func TestPool(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	_, err := gogosseract.NewPool(ctx, 5, gogosseract.PoolConfig{
		Config: gogosseract.Config{
			TrainingData: bytes.NewBuffer([]byte{}),
		},
	})
	if err == nil {
		t.Fatalf("gogosseract.NewPool should have failed")
	}

	pool, err := gogosseract.NewPool(ctx, 5, gogosseract.PoolConfig{
		Config: gogosseract.Config{
			TrainingData: bytes.NewBuffer(engTrainedData),
		},
	})
	test.FailOnError(t, err)
	defer pool.Close()

	images := [...]io.Reader{
		bytes.NewBuffer(logoImg), bytes.NewBuffer(docsImg),
		bytes.NewBuffer(logoImg), bytes.NewBuffer(docsImg),
		bytes.NewBuffer(logoImg), bytes.NewBuffer(docsImg),
		bytes.NewBuffer(logoImg), bytes.NewBuffer(docsImg),
		bytes.NewBuffer(logoImg), bytes.NewBuffer(docsImg),
	}

	textChan := make(chan string, len(images))
	for _, img := range images {
		go func(img io.Reader) {
			text, err := pool.ParseImage(ctx, img, gogosseract.ParseImageOptions{})
			if err != nil {
				panic(err)
			}
			textChan <- text
		}(img)
	}

	for range images {
		select {
		case <-ctx.Done():
			t.Fatal("timed out waiting for Pool.ParseImage")
		case r := <-textChan:
			if r != logoText && r != docsText {
				t.Fatalf("Pool.ParseImage returned unexpected text %s", r)
			}
		}
	}
	_, err = pool.ParseImage(ctx, nil, gogosseract.ParseImageOptions{})
	if err == nil {
		t.Fatalf("pool.ParseImage didn't return error")
	}
}
