//go:build ignore

package main

import (
	"archive/zip"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		fatalf("usage: go run ./scripts/check-android-artifact.go <aar|apk> <file> [--require-go-libs]")
	}
	kind := os.Args[1]
	path := os.Args[2]
	requireGoLibs := len(os.Args) > 3 && os.Args[3] == "--require-go-libs"
	zr, err := zip.OpenReader(path)
	if err != nil {
		fatalf("open %s: %v", path, err)
	}
	defer zr.Close()
	entries := map[string]bool{}
	for _, f := range zr.File {
		entries[f.Name] = true
	}
	switch kind {
	case "aar":
		require(entries, "classes.jar")
		require(entries, "AndroidManifest.xml")
		for _, abi := range []string{"arm64-v8a", "armeabi-v7a", "x86", "x86_64"} {
			require(entries, "jni/"+abi+"/libgojni.so")
		}
	case "apk":
		requireSuffix(entries, ".dex")
		require(entries, "AndroidManifest.xml")
		if requireGoLibs {
			for _, abi := range []string{"arm64-v8a", "armeabi-v7a", "x86", "x86_64"} {
				require(entries, "lib/"+abi+"/libgojni.so")
			}
		}
	default:
		fatalf("unknown artifact kind %q", kind)
	}
	fmt.Printf("%s artifact OK: %s\n", kind, path)
}

func require(entries map[string]bool, name string) {
	if !entries[name] {
		fatalf("missing %s", name)
	}
}

func requireSuffix(entries map[string]bool, suffix string) {
	for name := range entries {
		if strings.HasSuffix(name, suffix) {
			return
		}
	}
	fatalf("missing entry with suffix %s", suffix)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
