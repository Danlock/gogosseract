package wasm

import (
	"context"
	"log/slog"

	"github.com/danlock/pkg/errors"
	embind "github.com/jerbob92/wazero-emscripten-embind"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/emscripten"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// BuildImports implements or mocks the host imports required by Tesseract's compiled WASM module.
func BuildImports(ctx context.Context, waRT wazero.Runtime, embindEngine embind.Engine, compiledMod wazero.CompiledModule) error {
	if waRT.Module("wasi_snapshot_preview1") != nil {
		// If wasi_snapshot_preview1 was already instantiated, the same wazero runtime is being used for multiple Tesseract clients.
		return nil
	}

	wasi_snapshot_preview1.MustInstantiate(ctx, waRT)

	env := waRT.NewHostModuleBuilder("env")

	exporter, err := emscripten.NewFunctionExporterForModule(compiledMod)
	if err != nil {
		return errors.Errorf("emscripten.NewFunctionExporterForModule %w", err)
	}
	exporter.ExportFunctions(env)

	err = embindEngine.NewFunctionExporterForModule(compiledMod).ExportFunctions(env)
	if err != nil {
		return errors.Errorf("embind ExportFunctions %w", err)
	}

	// Even with -sFILESYSTEM=0 and -sPURE_WASI emscripten imports these syscalls and aborts them in JavaScript.
	// They should never get called, so they panic/no-op if they do.

	env.NewFunctionBuilder().WithFunc(func(ctx context.Context, mod api.Module, commandPtr int32) int32 {
		// http://pubs.opengroup.org/onlinepubs/000095399/functions/system.html
		// system is used to exec arbitrary commands. Lol, no.
		// Log the command string whatever is being attempted but this thankfully isn't called during our use of Tesseract.
		slog.Log(ctx, slog.LevelInfo, "env.system", "command", ReadString(mod.Memory(), commandPtr))
		// emscripten returns a 0 to indicate it's running in the browser without a shell. We shall do the same.
		return 0
	}).Export("system")

	env.NewFunctionBuilder().WithFunc(func(ctx context.Context, mod api.Module, buf, len int32) int32 {
		str, _ := mod.Memory().Read(uint32(buf), uint32(len))
		slog.Error("env.__syscall_getcwd", "buf", buf, "len", len, "str", string(str))
		panic("unimplemented host func")
	}).Export("__syscall_getcwd")

	env.NewFunctionBuilder().WithFunc(func(ctx context.Context, mod api.Module, dirfd, path, flags int32) int32 {
		slog.Error("env.__syscall_unlinkat", "dirfd", ReadString(mod.Memory(), dirfd),
			"path", ReadString(mod.Memory(), path), "flags", ReadString(mod.Memory(), flags))
		panic("unimplemented host func")
	}).Export("__syscall_unlinkat")

	env.NewFunctionBuilder().WithFunc(func(ctx context.Context, mod api.Module, path int32) int32 {
		slog.Error("env.__syscall_rmdir", "path", ReadString(mod.Memory(), path))
		panic("unimplemented host func")
	}).Export("__syscall_rmdir")

	env.NewFunctionBuilder().WithFunc(func(ctx context.Context, mod api.Module, fd, dirp, count int32) int32 {
		slog.Error("env.__syscall_getdents64", "fd", fd, "dirp", dirp, "count", count)
		panic("unimplemented host func")
	}).Export("__syscall_getdents64")

	env.NewFunctionBuilder().WithFunc(func(ctx context.Context, mod api.Module, dirfd, path, buf, bufsize int32) int32 {
		slog.Error("env.__syscall_readlinkat", "dirfd", dirfd, "path", ReadString(mod.Memory(), path), "buf", buf, "bufsize", bufsize)
		panic("unimplemented host func")
	}).Export("__syscall_readlinkat")

	_, err = env.Instantiate(ctx)
	return errors.Wrap(err)
}
