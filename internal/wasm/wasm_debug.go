//go:build gogosseract_debug

package wasm

import (
	_ "embed"
)

//go:embed tesseract-core-debug.wasm
var tesseractWASM []byte
