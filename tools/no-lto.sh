#!/usr/bin/env bash
# Rebuild libs/<triple>/libloro.a without LLVM bitcode.
#
# Go 1.27.4+ refuses LTO IR in cgo linker inputs (Mach-O: any __LLVM segment;
# ELF: .llvmbc / .llvmcmd). Every Rust object in these archives also carries
# machine code, so the IR can go without losing the C API. See PATCHED.md.
#
#   tools/no-lto.sh aarch64-apple-darwin        (macOS host)
#   tools/no-lto.sh x86_64-unknown-linux-musl   (Linux host, binutils)
set -euo pipefail

triple=${1:?usage: tools/no-lto.sh <triple>}
repo=$(pwd)
lib="libs/$triple/libloro.a"
[ -f "$lib" ] || { echo "no such archive: $lib" >&2; exit 1; }

# Count the exported C API: the one thing that must not change.
api() {
  nm --defined-only -g "$1" 2>/dev/null | grep -Ec ' T _?ffi_loro' || true
}

before=$(api "$lib")
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

case "$triple" in
*-apple-darwin)
  (cd "$work" && ar x "$repo/$lib")
  # ld -r merges the members into one relocatable object; the Apple linker
  # drops the __LLVM segment and keeps the machine code.
  xcrun ld -r -arch "$(uname -m)" -o "$work/merged.o" "$work"/*.o
  xcrun libtool -static -o "$lib" "$work/merged.o"
  ;;
*-linux-musl)
  (cd "$work" && ar x "$repo/$lib")
  mkdir "$work/out"
  for obj in "$work"/*.o; do
    objcopy --remove-section=.llvmbc --remove-section=.llvmcmd \
      "$obj" "$work/out/$(basename "$obj")"
  done
  rm -f "$lib"
  (cd "$work/out" && ar rcs "$repo/$lib" ./*.o)
  ;;
*)
  echo "unsupported triple: $triple" >&2
  exit 1
  ;;
esac

after=$(api "$lib")
if [ "$before" != "$after" ]; then
  echo "exported API changed: $before -> $after ffi_loro symbols" >&2
  exit 1
fi

# The Go gate scans every member, so no LTO section may remain anywhere.
if otool -l "$lib" 2>/dev/null | grep -q '__LLVM'; then
  echo "Mach-O __LLVM segment survived" >&2
  exit 1
fi
if readelf -S "$lib" 2>/dev/null | grep -qE '\.llvm(bc|cmd|\.lto)'; then
  echo "ELF LLVM section survived" >&2
  exit 1
fi

echo "ok: $triple carries no LTO IR, $after exported ffi_loro symbols intact"
