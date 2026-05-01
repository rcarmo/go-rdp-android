//go:build ignore

package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) != 3 {
		fatalf("usage: go run ./scripts/check-coverage.go <coverage-func.txt> <minimum-percent>")
	}
	min, err := strconv.ParseFloat(os.Args[2], 64)
	if err != nil {
		fatalf("invalid minimum: %v", err)
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		fatalf("open coverage summary: %v", err)
	}
	defer f.Close()
	var total float64
	found := false
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if !strings.HasPrefix(line, "total:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pct := strings.TrimSuffix(fields[len(fields)-1], "%")
		total, err = strconv.ParseFloat(pct, 64)
		if err != nil {
			fatalf("parse coverage %q: %v", pct, err)
		}
		found = true
	}
	if err := s.Err(); err != nil {
		fatalf("scan coverage summary: %v", err)
	}
	if !found {
		fatalf("total coverage line not found")
	}
	if total < min {
		fatalf("coverage %.1f%% below minimum %.1f%%", total, min)
	}
	fmt.Printf("coverage %.1f%% >= %.1f%%\n", total, min)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
