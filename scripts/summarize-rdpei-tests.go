//go:build ignore

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type testEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
	Output  string  `json:"Output"`
}

type testResult struct {
	Name    string
	Action  string
	Elapsed float64
}

var requiredRDPEITests = []string{
	"TestParseRDPEITouchEventSingleContact",
	"TestParseRDPEITouchEventOptionalFields",
	"TestDRDYNVCRDPEITouchClientSequenceIntegration",
	"TestDRDYNVCManagerPreservesRDPEIOptionalContactMetadata",
	"TestTouchLifecycleCoalescerPreservesOptionalMetadata",
}

func main() {
	if len(os.Args) != 3 {
		fatalf("usage: go run ./scripts/summarize-rdpei-tests.go <go-test-json.log> <summary.md>")
	}
	results, err := readResults(os.Args[1])
	if err != nil {
		fatalf("read test JSON: %v", err)
	}
	if err := writeSummary(os.Args[2], results); err != nil {
		fatalf("write summary: %v", err)
	}
}

func readResults(path string) (map[string]testResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	results := make(map[string]testResult)
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event testEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("parse %q: %w", line, err)
		}
		if event.Test == "" {
			continue
		}
		switch event.Action {
		case "pass", "fail", "skip":
			results[event.Test] = testResult{Name: event.Test, Action: event.Action, Elapsed: event.Elapsed}
		}
	}
	return results, scanner.Err()
}

func writeSummary(path string, results map[string]testResult) error {
	var b strings.Builder
	b.WriteString("# RDPEI / drdynvc test summary\n\n")
	b.WriteString("| Check | Status | Elapsed |\n")
	b.WriteString("| --- | --- | ---: |\n")
	allOK := true
	for _, name := range requiredRDPEITests {
		result, ok := results[name]
		status := "missing"
		elapsed := "-"
		if ok {
			status = result.Action
			elapsed = fmt.Sprintf("%.3fs", result.Elapsed)
		}
		if status != "pass" {
			allOK = false
		}
		b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n", name, status, elapsed))
	}
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("- Required RDPEI checks passed: `%t`\n", allOK))

	other := make([]string, 0)
	for name := range results {
		if strings.Contains(strings.ToLower(name), "rdpei") || strings.Contains(strings.ToLower(name), "drdynvc") || strings.Contains(strings.ToLower(name), "touchlifecycle") {
			if !isRequired(name) {
				other = append(other, name)
			}
		}
	}
	sort.Strings(other)
	if len(other) > 0 {
		b.WriteString("\n## Additional related checks\n\n")
		for _, name := range other {
			result := results[name]
			b.WriteString(fmt.Sprintf("- `%s`: `%s` (%.3fs)\n", name, result.Action, result.Elapsed))
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func isRequired(name string) bool {
	for _, required := range requiredRDPEITests {
		if name == required {
			return true
		}
	}
	return false
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
