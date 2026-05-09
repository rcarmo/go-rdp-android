//go:build ignore

package main

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		fatalf("usage: go run ./scripts/check-android-artifact.go <aar|apk|aab> <file> [--require-go-libs]")
	}
	kind := os.Args[1]
	path := os.Args[2]
	requireGoLibs := len(os.Args) > 3 && os.Args[3] == "--require-go-libs"
	zr, err := zip.OpenReader(path)
	if err != nil {
		fatalf("open %s: %v", path, err)
	}
	defer zr.Close()
	entries := map[string]*zip.File{}
	for _, f := range zr.File {
		entries[f.Name] = f
	}
	switch kind {
	case "aar":
		require(entries, "classes.jar")
		require(entries, "AndroidManifest.xml")
		for _, abi := range []string{"arm64-v8a", "armeabi-v7a", "x86", "x86_64"} {
			name := "jni/" + abi + "/libgojni.so"
			require(entries, name)
			checkELFPageAlignment(entries[name], name)
		}
	case "apk":
		requireSuffix(entries, ".dex")
		require(entries, "AndroidManifest.xml")
		if requireGoLibs {
			for _, abi := range []string{"arm64-v8a", "armeabi-v7a", "x86", "x86_64"} {
				name := "lib/" + abi + "/libgojni.so"
				require(entries, name)
				checkELFPageAlignment(entries[name], name)
			}
		}
	case "aab":
		require(entries, "BundleConfig.pb")
		require(entries, "base/manifest/AndroidManifest.xml")
		requireSuffixWithPrefix(entries, "base/dex/", ".dex")
		if requireGoLibs {
			for _, abi := range []string{"arm64-v8a", "armeabi-v7a", "x86", "x86_64"} {
				name := "base/lib/" + abi + "/libgojni.so"
				require(entries, name)
				checkELFPageAlignment(entries[name], name)
			}
		}
	default:
		fatalf("unknown artifact kind %q", kind)
	}
	fmt.Printf("%s artifact OK: %s\n", kind, path)
}

func require(entries map[string]*zip.File, name string) {
	if entries[name] == nil {
		fatalf("missing %s", name)
	}
}

func requireSuffix(entries map[string]*zip.File, suffix string) {
	for name := range entries {
		if strings.HasSuffix(name, suffix) {
			return
		}
	}
	fatalf("missing entry with suffix %s", suffix)
}

func requireSuffixWithPrefix(entries map[string]*zip.File, prefix, suffix string) {
	for name := range entries {
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix) {
			return
		}
	}
	fatalf("missing entry with prefix %s and suffix %s", prefix, suffix)
}

func checkELFPageAlignment(zf *zip.File, name string) {
	r, err := zf.Open()
	if err != nil {
		fatalf("open %s: %v", name, err)
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		fatalf("read %s: %v", name, err)
	}
	aligns, err := elfLoadAlignments(data)
	if err != nil {
		fatalf("parse %s: %v", name, err)
	}
	for _, align := range aligns {
		if align < 0x4000 || align%0x4000 != 0 {
			fatalf("%s is not 16 KB page compatible: PT_LOAD align=0x%x", name, align)
		}
	}
}

func elfLoadAlignments(data []byte) ([]uint64, error) {
	if len(data) < 64 || string(data[:4]) != "\x7fELF" {
		return nil, fmt.Errorf("not an ELF file")
	}
	var order binary.ByteOrder
	switch data[5] {
	case 1:
		order = binary.LittleEndian
	case 2:
		order = binary.BigEndian
	default:
		return nil, fmt.Errorf("unknown ELF byte order %d", data[5])
	}
	var phoff uint64
	var phentsize, phnum uint16
	is64 := data[4] == 2
	if is64 {
		phoff = order.Uint64(data[32:40])
		phentsize = order.Uint16(data[54:56])
		phnum = order.Uint16(data[56:58])
	} else if data[4] == 1 {
		phoff = uint64(order.Uint32(data[28:32]))
		phentsize = order.Uint16(data[42:44])
		phnum = order.Uint16(data[44:46])
	} else {
		return nil, fmt.Errorf("unknown ELF class %d", data[4])
	}
	if phentsize == 0 || phnum == 0 {
		return nil, fmt.Errorf("missing program headers")
	}
	if phoff+uint64(phentsize)*uint64(phnum) > uint64(len(data)) {
		return nil, fmt.Errorf("program headers exceed file size")
	}
	var aligns []uint64
	for i := 0; i < int(phnum); i++ {
		off := int(phoff) + i*int(phentsize)
		ph := data[off : off+int(phentsize)]
		pType := order.Uint32(ph[0:4])
		if pType != 1 { // PT_LOAD
			continue
		}
		var align uint64
		if is64 {
			if len(ph) < 56 {
				return nil, fmt.Errorf("short ELF64 program header")
			}
			align = order.Uint64(ph[48:56])
		} else {
			if len(ph) < 32 {
				return nil, fmt.Errorf("short ELF32 program header")
			}
			align = uint64(order.Uint32(ph[28:32]))
		}
		aligns = append(aligns, align)
	}
	if len(aligns) == 0 {
		return nil, fmt.Errorf("no PT_LOAD segments")
	}
	return aligns, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
