//go:build ignore

package main

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	if len(os.Args) != 4 {
		fatalf("usage: go run ./scripts/summarize-android-artifact.go <aar|apk|aab> <file> <summary.md>")
	}
	kind, path, out := os.Args[1], os.Args[2], os.Args[3]
	zr, err := zip.OpenReader(path)
	if err != nil {
		fatalf("open %s: %v", path, err)
	}
	defer zr.Close()
	var entries []string
	abis := map[string]bool{}
	var dex, manifest, classesJar, bundleConfig bool
	for _, f := range zr.File {
		entries = append(entries, f.Name)
		if f.Name == "AndroidManifest.xml" || f.Name == "base/manifest/AndroidManifest.xml" {
			manifest = true
		}
		if strings.HasSuffix(f.Name, ".dex") {
			dex = true
		}
		if f.Name == "classes.jar" {
			classesJar = true
		}
		if f.Name == "BundleConfig.pb" {
			bundleConfig = true
		}
		parts := strings.Split(f.Name, "/")
		if len(parts) >= 3 && (parts[0] == "lib" || parts[0] == "jni") && parts[len(parts)-1] == "libgojni.so" {
			abis[parts[1]] = true
		}
		if len(parts) >= 4 && parts[0] == "base" && parts[1] == "lib" && parts[len(parts)-1] == "libgojni.so" {
			abis[parts[2]] = true
		}
	}
	sort.Strings(entries)
	st, _ := os.Stat(path)
	md := fmt.Sprintf("# %s artifact summary\n\n- File: `%s`\n- Size: `%d` bytes\n- Manifest: `%v`\n- DEX present: `%v`\n- classes.jar present: `%v`\n- BundleConfig.pb present: `%v`\n- Go JNI ABIs: `%s`\n- Entry count: `%d`\n\n## First entries\n\n", strings.ToUpper(kind), filepath.Base(path), st.Size(), manifest, dex, classesJar, bundleConfig, strings.Join(keys(abis), ", "), len(entries))
	limit := 80
	if len(entries) < limit {
		limit = len(entries)
	}
	for _, e := range entries[:limit] {
		md += "- `" + e + "`\n"
	}
	if len(entries) > limit {
		md += fmt.Sprintf("- ... %d more\n", len(entries)-limit)
	}
	if err := os.WriteFile(out, []byte(md), 0o644); err != nil {
		fatalf("write summary: %v", err)
	}
	fmt.Println("wrote", out)
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return []string{"none"}
	}
	return out
}
func fatalf(format string, args ...any) { fmt.Fprintf(os.Stderr, format+"\n", args...); os.Exit(1) }
