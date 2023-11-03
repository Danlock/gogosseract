//go:build !gogosseract_debug

package wasm

import (
	_ "embed"
)

//go:embed tesseract-core.wasm
var tesseractWASM []byte
