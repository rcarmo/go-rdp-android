package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var requiredSecrets = []string{
	"RELEASE_KEYSTORE_BASE64",
	"RELEASE_KEYSTORE_PASSWORD",
	"RELEASE_KEY_ALIAS",
	"RELEASE_KEY_PASSWORD",
}

func main() {
	repo := flag.String("repo", "rcarmo/go-rdp-android", "GitHub repository owner/name")
	requireSecrets := flag.Bool("require-secrets", true, "fail when required GitHub Actions signing secrets are not visible")
	flag.Parse()

	checks := []struct {
		name string
		fn   func() error
	}{
		{"git working tree clean", checkGitClean},
		{"local branch synced with upstream", checkGitSynced},
		{"release version identifiers aligned", checkVersionAlignment},
		{"latest GitHub Actions run green", func() error { return checkLatestCI(*repo) }},
		{"GitHub Actions signing secrets visible", func() error { return checkSigningSecrets(*repo, *requireSecrets) }},
	}

	failed := false
	for _, check := range checks {
		if err := check.fn(); err != nil {
			failed = true
			fmt.Printf("[FAIL] %s: %v\n", check.name, err)
			continue
		}
		fmt.Printf("[ OK ] %s\n", check.name)
	}
	if failed {
		os.Exit(1)
	}
}

func checkGitClean() error {
	out, err := runGit("status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("working tree has changes:\n%s", out)
	}
	return nil
}

func checkGitSynced() error {
	if _, err := runGit("fetch", "--quiet"); err != nil {
		return err
	}
	out, err := runGit("rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err != nil {
		return err
	}
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return fmt.Errorf("unexpected rev-list output %q", strings.TrimSpace(out))
	}
	if fields[0] != "0" || fields[1] != "0" {
		return fmt.Errorf("branch diverged from upstream: ahead=%s behind=%s", fields[0], fields[1])
	}
	return nil
}

func checkVersionAlignment() error {
	versionBytes, err := os.ReadFile("VERSION")
	if err != nil {
		return err
	}
	version := strings.TrimSpace(string(versionBytes))
	if version == "" {
		return fmt.Errorf("VERSION is empty")
	}
	gradle, err := os.ReadFile("android/app/build.gradle.kts")
	if err != nil {
		return err
	}
	versionName := firstSubmatch(string(gradle), `versionName\s*=\s*"([^"]+)"`)
	versionCode := firstSubmatch(string(gradle), `versionCode\s*=\s*(\d+)`)
	if versionName != version {
		return fmt.Errorf("VERSION=%q but Android versionName=%q", version, versionName)
	}
	if versionCode == "" {
		return fmt.Errorf("Android versionCode not found")
	}
	return nil
}

func checkLatestCI(repo string) error {
	out, err := runGh("run", "list", "--repo", repo, "--branch", "main", "--limit", "1", "--json", "status,conclusion,databaseId,headSha,displayTitle", "--jq", `.[0] | [.databaseId, .status, (.conclusion // ""), .headSha, .displayTitle] | @tsv`)
	if err == nil {
		fields := strings.Split(strings.TrimSpace(out), "\t")
		if len(fields) < 5 {
			return fmt.Errorf("unexpected gh run output %q", strings.TrimSpace(out))
		}
		if fields[1] != "completed" || fields[2] != "success" {
			return fmt.Errorf("latest run %s (%s) status=%s conclusion=%s", fields[0], fields[4], fields[1], fields[2])
		}
		return nil
	}
	return checkLatestCIWithGitHubAPI(repo, err)
}

func checkSigningSecrets(repo string, require bool) error {
	out, err := runGh("secret", "list", "--repo", repo)
	visible := map[string]bool{}
	if err == nil {
		for _, line := range strings.Split(out, "\n") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				visible[fields[0]] = true
			}
		}
	} else {
		apiVisible, apiErr := githubAPISecretNames(repo)
		if apiErr != nil {
			return fmt.Errorf("gh unavailable (%v) and GitHub API fallback failed: %w", err, apiErr)
		}
		for _, name := range apiVisible {
			visible[name] = true
		}
	}
	missing := []string{}
	for _, secret := range requiredSecrets {
		if !visible[secret] {
			missing = append(missing, secret)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	msg := fmt.Sprintf("missing or not visible: %s", strings.Join(missing, ", "))
	if require {
		return fmt.Errorf("%s", msg)
	}
	fmt.Printf("[WARN] GitHub Actions signing secrets: %s\n", msg)
	return nil
}

type githubRunsResponse struct {
	WorkflowRuns []struct {
		ID           int64  `json:"id"`
		Status       string `json:"status"`
		Conclusion   string `json:"conclusion"`
		HeadSHA      string `json:"head_sha"`
		DisplayTitle string `json:"display_title"`
	} `json:"workflow_runs"`
}

type githubSecretsResponse struct {
	Secrets []struct {
		Name string `json:"name"`
	} `json:"secrets"`
}

func checkLatestCIWithGitHubAPI(repo string, ghErr error) error {
	var runs githubRunsResponse
	if err := githubAPIGet(repo, "/actions/runs?branch=main&per_page=1", &runs); err != nil {
		return fmt.Errorf("gh unavailable (%v) and GitHub API fallback failed: %w", ghErr, err)
	}
	if len(runs.WorkflowRuns) == 0 {
		return fmt.Errorf("no GitHub Actions runs found for %s main", repo)
	}
	run := runs.WorkflowRuns[0]
	if run.Status != "completed" || run.Conclusion != "success" {
		return fmt.Errorf("latest run %d (%s) status=%s conclusion=%s", run.ID, run.DisplayTitle, run.Status, run.Conclusion)
	}
	return nil
}

func githubAPISecretNames(repo string) ([]string, error) {
	var secrets githubSecretsResponse
	if err := githubAPIGet(repo, "/actions/secrets?per_page=100", &secrets); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(secrets.Secrets))
	for _, secret := range secrets.Secrets {
		names = append(names, secret.Name)
	}
	return names, nil
}

func githubAPIGet(repo string, path string, out any) error {
	token := firstNonEmpty(os.Getenv("GITHUB_TOKEN"), os.Getenv("GH_TOKEN"), os.Getenv("GITHUB_PICLAW_BOT"))
	if token == "" {
		return fmt.Errorf("no GITHUB_TOKEN, GH_TOKEN, or GITHUB_PICLAW_BOT set")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+repo+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub API %s returned %s", path, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return err
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstSubmatch(s, pattern string) string {
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(s)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...) // #nosec G204 -- release preflight invokes a fixed git binary with fixed call sites.
	return runCommand(cmd)
}

func runGh(args ...string) (string, error) {
	cmd := exec.Command("gh", args...) // #nosec G204 -- release preflight invokes a fixed gh binary with fixed call sites.
	return runCommand(cmd)
}

func runCommand(cmd *exec.Cmd) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		cmdline := strings.Join(cmd.Args, " ")
		if msg != "" {
			return stdout.String(), fmt.Errorf("%s: %w: %s", cmdline, err, msg)
		}
		return stdout.String(), fmt.Errorf("%s: %w", cmdline, err)
	}
	return stdout.String(), nil
}
