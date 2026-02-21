module github.com/danlock/gogosseract

go 1.24.0

require (
	github.com/danlock/pkg v0.0.18-fc7c42d
	github.com/google/go-cmp v0.6.0
	github.com/jerbob92/wazero-emscripten-embind v1.3.0
	github.com/tetratelabs/wazero v1.11.0
	golang.org/x/exp v0.0.0-20231006140011-7918f672742d
	golang.org/x/sync v0.4.0
)

require (
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.13.0 // indirect
)

replace github.com/tetratelabs/wazero => github.com/jerbob92/wazero v0.0.0-20240718051810-c077eabacdef
