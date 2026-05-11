//go:build ignore

package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"
)

var required = map[string]map[string]string{
	"mobile/Mobile.class": {
		"addr":                 "()Ljava/lang/String;",
		"startServer":          "(J)V",
		"stopServer":           "()V",
		"submitFrame":          "(JJJJ[B)V",
		"setInputHandler":      "(Lmobile/InputHandler;)V",
		"tlsFingerprintSHA256": "()Ljava/lang/String;",
	},
	"mobile/InputHandler.class": {
		"pointerMove":   "(JJ)V",
		"pointerButton": "(JJJZ)V",
		"pointerWheel":  "(JJJZ)V",
		"key":           "(JZ)V",
		"unicode":       "(J)V",
		"touchContact":  "(JJJJ)V",
	},
	"mobile/Server.class": {
		"addr":                 "()Ljava/lang/String;",
		"start":                "(J)V",
		"stop":                 "()V",
		"submitFrame":          "(JJJJ[B)V",
		"setInputHandler":      "(Lmobile/InputHandler;)V",
		"tlsFingerprintSHA256": "()Ljava/lang/String;",
	},
	"mobile/FrameQueue.class": {
		"close": "()V",
	},
}

func main() {
	if len(os.Args) != 2 {
		fatalf("usage: go run ./scripts/check-aar-api.go <mobile.aar>")
	}
	classes, err := readNestedZip(os.Args[1], "classes.jar")
	if err != nil {
		fatalf("read classes.jar: %v", err)
	}
	for className, methods := range required {
		data, err := readZipEntry(classes, className)
		if err != nil {
			fatalf("read %s: %v", className, err)
		}
		got, err := parseClassMethods(data)
		if err != nil {
			fatalf("parse %s: %v", className, err)
		}
		for name, desc := range methods {
			if got[name] != desc {
				fatalf("%s missing %s%s; got %q", className, name, desc, got[name])
			}
		}
	}
	fmt.Println("mobile AAR API OK")
}

func readNestedZip(path, name string) ([]byte, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == name {
			return readFile(f)
		}
	}
	return nil, fmt.Errorf("%s not found", name)
}

func readZipEntry(zipData []byte, name string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if f.Name == name {
			return readFile(f)
		}
	}
	return nil, fmt.Errorf("%s not found", name)
}

func readFile(f *zip.File) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func parseClassMethods(data []byte) (map[string]string, error) {
	if len(data) < 10 || binary.BigEndian.Uint32(data[0:4]) != 0xcafebabe {
		return nil, fmt.Errorf("not a Java class file")
	}
	cpCount := int(binary.BigEndian.Uint16(data[8:10]))
	cp := make([]any, cpCount)
	off := 10
	for i := 1; i < cpCount; i++ {
		if off >= len(data) {
			return nil, io.ErrUnexpectedEOF
		}
		tag := data[off]
		off++
		switch tag {
		case 1: // Utf8
			if off+2 > len(data) {
				return nil, io.ErrUnexpectedEOF
			}
			l := int(binary.BigEndian.Uint16(data[off : off+2]))
			off += 2
			if off+l > len(data) {
				return nil, io.ErrUnexpectedEOF
			}
			cp[i] = string(data[off : off+l])
			off += l
		case 3, 4:
			off += 4
		case 5, 6:
			off += 8
			i++
		case 7, 8, 16, 19, 20:
			off += 2
		case 9, 10, 11, 12, 18:
			off += 4
		case 15:
			off += 3
		default:
			return nil, fmt.Errorf("unknown constant tag %d", tag)
		}
	}
	if off+8 > len(data) {
		return nil, io.ErrUnexpectedEOF
	}
	off += 6 // access_flags, this_class, super_class
	interfaces := int(binary.BigEndian.Uint16(data[off : off+2]))
	off += 2 + 2*interfaces
	fields := int(binary.BigEndian.Uint16(data[off : off+2]))
	off += 2
	for i := 0; i < fields; i++ {
		off += 6
		attrs := int(binary.BigEndian.Uint16(data[off : off+2]))
		off += 2
		for j := 0; j < attrs; j++ {
			off += 2
			l := int(binary.BigEndian.Uint32(data[off : off+4]))
			off += 4 + l
		}
	}
	methods := int(binary.BigEndian.Uint16(data[off : off+2]))
	off += 2
	out := map[string]string{}
	for i := 0; i < methods; i++ {
		off += 2 // access_flags
		nameIdx := int(binary.BigEndian.Uint16(data[off : off+2]))
		off += 2
		descIdx := int(binary.BigEndian.Uint16(data[off : off+2]))
		off += 2
		name, _ := cp[nameIdx].(string)
		desc, _ := cp[descIdx].(string)
		out[name] = desc
		attrs := int(binary.BigEndian.Uint16(data[off : off+2]))
		off += 2
		for j := 0; j < attrs; j++ {
			off += 2
			l := int(binary.BigEndian.Uint32(data[off : off+4]))
			off += 4 + l
		}
	}
	return out, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func init() {
	for _, methods := range required {
		names := make([]string, 0, len(methods))
		for name := range methods {
			names = append(names, name)
		}
		sort.Strings(names)
	}
}
