module github.com/danlock/gogosseract

go 1.21.0

require (
	github.com/google/go-cmp v0.5.9
	github.com/jerbob92/wazero-emscripten-embind v1.2.1
	github.com/tetratelabs/wazero v1.5.0
	golang.org/x/exp v0.0.0-20231006140011-7918f672742d
)

replace github.com/jerbob92/wazero-emscripten-embind => ../wazero-emscripten-embind

require (
	golang.org/x/net v0.16.0 // indirect
	golang.org/x/text v0.13.0 // indirect
)
