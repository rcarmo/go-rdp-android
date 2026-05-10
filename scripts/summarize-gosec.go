//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

type gosecReport struct {
	Issues []struct {
		RuleID     string `json:"rule_id"`
		Severity   string `json:"severity"`
		Confidence string `json:"confidence"`
		File       string `json:"file"`
		Line       string `json:"line"`
		Details    string `json:"details"`
	} `json:"Issues"`
	Stats struct {
		Files int `json:"files"`
		Lines int `json:"lines"`
		NoSec int `json:"nosec"`
		Found int `json:"found"`
	} `json:"Stats"`
	GosecVersion string `json:"GosecVersion"`
}

func main() {
	if len(os.Args) != 3 {
		fatalf("usage: go run ./scripts/summarize-gosec.go <report.json> <summary.md>")
	}
	inPath := os.Args[1]
	outPath := os.Args[2]
	data, err := os.ReadFile(inPath)
	if err != nil {
		fatalf("read %s: %v", inPath, err)
	}
	var report gosecReport
	if err := json.Unmarshal(data, &report); err != nil {
		fatalf("parse %s: %v", inPath, err)
	}

	ruleCounts := map[string]int{}
	for _, issue := range report.Issues {
		ruleCounts[issue.RuleID]++
	}
	rules := make([]string, 0, len(ruleCounts))
	for rule := range ruleCounts {
		rules = append(rules, rule)
	}
	sort.Strings(rules)

	md := "# gosec summary\n\n"
	md += fmt.Sprintf("- files: %d\n", report.Stats.Files)
	md += fmt.Sprintf("- lines: %d\n", report.Stats.Lines)
	md += fmt.Sprintf("- findings: %d\n", report.Stats.Found)
	md += fmt.Sprintf("- nosec annotations: %d\n", report.Stats.NoSec)
	md += fmt.Sprintf("- gosec version: %s\n\n", report.GosecVersion)

	if len(rules) == 0 {
		md += "No findings.\n"
	} else {
		md += "## Findings by rule\n\n"
		for _, rule := range rules {
			md += fmt.Sprintf("- %s: %d\n", rule, ruleCounts[rule])
		}
		md += "\n## First findings\n\n"
		limit := len(report.Issues)
		if limit > 20 {
			limit = 20
		}
		for _, issue := range report.Issues[:limit] {
			md += fmt.Sprintf("- `%s` %s:%s (%s/%s) — %s\n", issue.RuleID, issue.File, issue.Line, issue.Severity, issue.Confidence, issue.Details)
		}
	}

	if err := os.WriteFile(outPath, []byte(md), 0o644); err != nil {
		fatalf("write %s: %v", outPath, err)
	}
	fmt.Println("wrote", outPath)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
