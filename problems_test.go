package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func putTestFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

const checkerTestPrefix = "{\r\n    \"md5_check\": [ { \"name\" : \"\", \"md5\" : \"\" } ],\r\n    \"problems\": "
const checkerTestSuffix = ",\r\n    " + `"problem_regex": "^{0}\\\\{0}\\.(cpp|c|pas)$",` + "\r\n    \"problem_extern\": false\r\n}\r\n"
const emptyCheckerFixture = checkerTestPrefix + "[]" + checkerTestSuffix

func readFixtureProblems(t *testing.T, base string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(base, checkerConfigName))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.HasPrefix(content, checkerTestPrefix) || !strings.HasSuffix(content, checkerTestSuffix) {
		t.Fatalf("changed configuration outside problems: %s", content)
	}
	raw := content[len(checkerTestPrefix) : len(content)-len(checkerTestSuffix)]
	var entries []struct{ Name, Regex string }
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		name := regexp.QuoteMeta(e.Name)
		want := "^" + name + `\\` + name + `\.(cpp|c|pas)$`
		if e.Regex != want {
			t.Fatalf("regex for %q = %q; want %q", e.Name, e.Regex, want)
		}
		names = append(names, e.Name)
	}
	return names
}

func TestScanAndPublishCheckerProblems(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(t.TempDir(), "stage")
	for _, stem := range []string{
		"外层 目录/子任务一/sum01",
		"外层 目录/子任务二/sUm_2",
		"资料/01sum",
		"资料/ab2cd3",
		"graph/long-Path_9",
		"graph/magic_word.1",
		"misleading-folder/a1_2",
		"plain",
	} {
		putTestFile(t, root, stem+".IN", "")
		putTestFile(t, root, stem+".AnS", "")
	}
	putTestFile(t, root, "readme.txt", "ignore")
	putTestFile(t, root, "only_input/dice03.in", "")
	putTestFile(t, root, "other_place/dice03.ans", "")
	putTestFile(t, root, "numeric/01.in", "")
	putTestFile(t, root, "orphan.ans", "")
	putTestFile(t, root, checkerConfigName, "from archive")
	putTestFile(t, base, checkerConfigName, emptyCheckerFixture)
	putTestFile(t, base, "problem.extern", "legacy manifest")
	if err := os.Chmod(filepath.Join(base, checkerConfigName), 0444); err != nil {
		t.Fatal(err)
	}
	summary, err := scanProblems(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "abcd", "dice", "long-Path", "magic_word", "plain", "sum"}
	wantSkipped := []string{filepath.Join("numeric", "01.in")}
	if summary.Inputs != 10 || summary.Paired != 8 || !reflect.DeepEqual(summary.Problems, want) ||
		!reflect.DeepEqual(summary.Skipped, wantSkipped) {
		t.Fatalf("unexpected summary %+v", summary)
	}
	dest, err := publishResult(base, root, summary.Problems, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(filepath.Join(dest, checkerConfigName))
	if err != nil || string(original) != "from archive" {
		t.Fatalf("changed archive member %q, err %v", original, err)
	}
	if got := readFixtureProblems(t, base); !reflect.DeepEqual(got, want) {
		t.Fatalf("problems = %v; want %v", got, want)
	}
	info, err := os.Stat(filepath.Join(base, checkerConfigName))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0200 != 0 {
		t.Fatal("read-only checker configuration became writable")
	}
	if body, err := os.ReadFile(filepath.Join(base, "problem.extern")); err != nil || string(body) != "legacy manifest" {
		t.Fatalf("changed old problem.extern: %q, %v", body, err)
	}
}

func TestScanRejectsInvalidData(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paths []string
		want  string
	}{
		{"empty", []string{"readme"}, "没有找到"},
		{"only answer files", []string{"tree01.ans"}, ".in"},
		{"numeric only", []string{"tree/1.in", "tree/1.ans"}, "英文题名"},
		{"no English name", []string{"题目01.in"}, "英文题名"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for _, p := range tc.paths {
				putTestFile(t, root, p, "")
			}
			_, err := scanProblems(root)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v, want %q", err, tc.want)
			}
		})
	}
}

func TestReplaceCheckerConfigPreservesLinkTarget(t *testing.T) {
	root := t.TempDir()
	otherDir := t.TempDir()
	target := filepath.Join(otherDir, "keep.txt")
	putTestFile(t, otherDir, "keep.txt", "untouched")
	putTestFile(t, root, "new-checker", emptyCheckerFixture)
	dest := filepath.Join(root, checkerConfigName)
	if err := os.Symlink(target, dest); err != nil {
		t.Skipf("cannot create a symbolic link on this system: %v", err)
	}
	if err := replaceCheckerConfig(filepath.Join(root, "new-checker"), dest); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(target)
	if err != nil || string(body) != "untouched" {
		t.Fatalf("link target %q, err %v", body, err)
	}
}

func TestPublishResultsWithinOneSecond(t *testing.T) {
	base := t.TempDir()
	putTestFile(t, base, checkerConfigName, emptyCheckerFixture)
	stamp := time.Date(2026, 11, 24, 14, 30, 52, 0, time.Local)
	first := "down20261124_143052"
	if err := os.Mkdir(filepath.Join(base, first), 0700); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	done := make(chan error, 2)
	for _, name := range []string{"red", "blue"} {
		stage := filepath.Join(t.TempDir(), "stage")
		putTestFile(t, stage, name+".in", name)
		go func() {
			<-start
			dest, err := publishResult(base, stage, []string{name}, stamp)
			if err == nil {
				got, readErr := os.ReadFile(filepath.Join(dest, name+".in"))
				if string(got) != name || readErr != nil {
					err = fmt.Errorf("result contains %q, read error %v", got, readErr)
				}
			}
			done <- err
		}()
	}
	close(start)
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if entries, err := os.ReadDir(filepath.Join(base, first)); err != nil || len(entries) != 0 {
		t.Fatalf("preexisting folder changed: %v, %v", entries, err)
	}
	for _, suffix := range []string{"_2", "_3"} {
		if _, err := os.Stat(filepath.Join(base, first+suffix)); err != nil {
			t.Fatal(err)
		}
	}
	if got := readFixtureProblems(t, base); len(got) != 1 || (got[0] != "red" && got[0] != "blue") {
		t.Fatalf("incomplete checker configuration: %v", got)
	}
}

func TestPublishPreservesInvalidOrMissingChecker(t *testing.T) {
	for _, tc := range []struct{ name, existing, content string }{
		{"missing", "", ""},
		{"invalid JSON", checkerConfigName, `{"problems":[}`},
		{"directory", checkerConfigName + "/keep.txt", "keep"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			if tc.existing != "" {
				putTestFile(t, base, tc.existing, tc.content)
			}
			stage := filepath.Join(t.TempDir(), "stage")
			putTestFile(t, stage, "sum1.in", "1")
			if _, err := publishResult(base, stage, []string{"sum"}, time.Now()); err == nil {
				t.Fatal("expected publication to fail")
			}
			entries, err := os.ReadDir(base)
			if err != nil {
				t.Fatal(err)
			}
			if tc.existing == "" {
				if len(entries) != 0 {
					t.Fatalf("created unexpected output: %v", entries)
				}
				return
			}
			if len(entries) != 1 || entries[0].Name() != checkerConfigName {
				t.Fatalf("partial publication: %v", entries)
			}
			if got, err := os.ReadFile(filepath.Join(base, tc.existing)); err != nil || string(got) != tc.content {
				t.Fatalf("original content changed: %q, %v", got, err)
			}
		})
	}
}
