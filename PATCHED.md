# This fork ships bitcode-free archives

Upstream: `github.com/aholstenson/loro-go` at **v0.5.0** (`b51c3b2d`,
2026-06-24). The Go code here is upstream's, unmodified; only the prebuilt
`libs/<triple>/libloro.a` archives differ.

## Why

Go 1.27.4 tightened cgo linker inputs: the go command now refuses to pass a
library to the C linker if it contains LTO intermediate representation — any
`__LLVM` segment in a Mach-O archive member, or `.llvmbc` / `.llvmcmd`
sections in an ELF one (the CVE-2026-97033…97043 cgo hardening). The archives
upstream publishes were built with fat LTO, so every member carries both
bitcode and machine code and a cgo build fails with:

```
disallowed LTO section __LLVM,__bitcode in #cgo LDFLAGS: .../libloro.a
```

The bitcode is redundant here: the machine code is present in the same
objects, so dropping the IR keeps the C API and the linker happy.

## What changed

| triple | how |
|---|---|
| `aarch64-apple-darwin` | members merged with `ld -r` (drops the `__LLVM` segment), re-archived with `libtool -static` |
| `x86_64-unknown-linux-musl` | `.llvmbc` / `.llvmcmd` removed per member with `objcopy`, re-archived with `ar rcs` |
| `aarch64-unknown-linux-musl` | same as x86_64 |

`x86_64-apple-darwin` and `x86_64-pc-windows-gnu` are untouched: they are not
built by the consumer of this fork, and they still carry bitcode.

`tools/no-lto.sh <triple>` reproduces the transform and fails if the exported
`ffi_loro_*` API count changes or any LTO section survives.

## How to stop using this fork

When upstream publishes libs built without LTO (or a Go release stops
rejecting them), delete the `replace` line pointing here and pin upstream
again:

```
go get github.com/aholstenson/loro-go@latest
```

Nothing else in the consumer depends on this fork.

## Provenance

- Base commit: `b51c3b2d` (`v0.5.0`), upstream `main`.
- Tag `v0.5.1` exists only to give the `replace` a version to pin.
- Verified on macOS arm64: `go build ./cmd/web` and the notes browser suite
  pass with Go 1.27.4, and `govulncheck` reports no affecting vulnerabilities.
