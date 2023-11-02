# gogosseract
A reimplementation of https://github.com/otiai10/gosseract without CGo, running Tesseract compiled to WASM with Wazero

The WASM is generated from my [personal](https://github.com/Danlock/tesseract-wasm) fork of robertknight's well written tesseract-wasm project.

# Training Data

Tesseract requires training data in order to accurately recognize text. The official source is [here](https://github.com/tesseract-ocr/tessdata_fast). Strategies for dealing with this include downloading it at runtime, or embedding the file within your Go binary using go:embed at compile time.