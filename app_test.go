package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestConfigWindowsNames(t *testing.T) {
	base := t.TempDir()
	const template = "# 配置\r\nouter_archive = 外层 文件.zip\r\ninner_archive = 试题\\内层.7z\r\n"
	for _, tc := range []struct {
		name string
		data string
		ok   bool
	}{
		{"UTF-8 CRLF and nested inner path", template, true},
		{"legacy UTF-8 BOM", "\ufeff" + template, true},
		{"legacy output directory", template + "output_dir = 结果\n", true},
		{"duplicate", template + "outer_archive = newer.7z\n", false},
		{"escaping path", strings.Replace(template, "试题\\内层.7z", "../内层.7z", 1), false},
		{"outer directory", strings.Replace(template, "外层 文件.zip", "a\\b.zip", 1), false},
		{"missing field", "inner_archive = inner.7z", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(base, configName), []byte(tc.data), 0600); err != nil {
				t.Fatal(err)
			}
			c, err := loadConfig(base)
			if (err == nil) != tc.ok {
				t.Fatalf("config = %+v, error = %v", c, err)
			}
		})
	}
}

func TestRunPublishesCompleteResult(t *testing.T) {
	exe := os.Getenv("CSP_TEST_7Z")
	if exe == "" {
		t.Skip("set CSP_TEST_7Z to run the full extraction workflow")
	}
	base := t.TempDir()
	const innerPW, outerPW = " int & ! ", "ext 空格"
	createFixture := func(names ...string) {
		t.Helper()
		source := t.TempDir()
		testPath := filepath.Join(source, "测试 数据")
		if err := os.Mkdir(testPath, 0700); err != nil {
			t.Fatal(err)
		}
		for _, name := range names {
			if err := os.WriteFile(filepath.Join(testPath, name), []byte("1\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		for _, args := range [][]string{
			{"a", "-mem=AES256", "-p" + innerPW, "数据.zip", "测试 数据"},
			{"a", "-mhe=on", "-p" + outerPW, "外层.7z", "数据.zip"},
		} {
			cmd := exec.Command(exe, args...)
			cmd.Dir = source
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("fixture: %v: %s", err, out)
			}
		}
		if err := os.Rename(filepath.Join(source, "外层.7z"), filepath.Join(base, "外层.7z")); err != nil {
			t.Fatal(err)
		}
	}
	createFixture("sum01.in", "sum01.ans", "sum2.in", "path-1.IN")
	const cfg = "outer_archive=外层.7z\ninner_archive=数据.zip\noutput_dir=结果"
	if err := os.WriteFile(filepath.Join(base, configName), []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}
	// A stale sibling with the inner archive's name must never be used.
	if err := os.WriteFile(filepath.Join(base, "数据.zip"), []byte("stale"), 0600); err != nil {
		t.Fatal(err)
	}
	simulate := func(passwords ...string) (string, error) {
		t.Helper()
		ask := func(string) ([]byte, error) {
			if len(passwords) == 0 {
				return nil, io.EOF
			}
			p := passwords[0]
			passwords = passwords[1:]
			return []byte(p), nil
		}
		var log bytes.Buffer
		err := run(context.Background(), base, ask, &log, func(string) (string, error) { return exe, nil })
		if leftovers, _ := filepath.Glob(filepath.Join(base, ".csp-unzip-*")); len(leftovers) != 0 {
			t.Fatalf("temporary directories still present: %v", leftovers)
		}
		return log.String(), err
	}
	results := func() []string {
		t.Helper()
		matches, err := filepath.Glob(filepath.Join(base, "down*"))
		if err != nil {
			t.Fatal(err)
		}
		return matches
	}
	checkContents := func(path, want string) {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Fatalf("reading %s: got %q, want %q, error = %v", path, data, want, err)
		}
	}
	manifest := filepath.Join(base, "problem.extern")
	log, err := simulate(outerPW, innerPW)
	if err != nil || strings.Contains(log, innerPW) || strings.Contains(log, outerPW) {
		t.Fatalf("single-entry extraction failed: %v", err)
	}
	checkContents(manifest, "path\r\nsum\r\n")
	first := results()
	if len(first) != 1 {
		t.Fatalf("first run: output directories = %v", first)
	}
	if _, err := os.Stat(filepath.Join(first[0], "problem.extern")); !os.IsNotExist(err) {
		t.Fatalf("unexpected manifest in extracted data, err = %v", err)
	}
	createFixture("tree03.in", "tree03.ans")
	log, err = simulate("wrong", outerPW, "wrong", innerPW)
	if err != nil {
		t.Fatal(err)
	}
	for _, layer := range []string{"外层", "内层"} {
		if strings.Count(log, layer+"密码错误，请重新输入。") != 1 {
			t.Fatalf("expected one retry message for %s, got %q", layer, log)
		}
	}
	checkContents(manifest, "tree\r\n")
	both := results()
	if len(both) != 2 || !slices.Contains(both, first[0]) {
		t.Fatalf("repeat run: output directories = %v, first = %v", both, first)
	}
	if _, err := simulate(outerPW, "wrong"); !errors.Is(err, io.EOF) {
		t.Fatalf("expected end of input while retrying, got %v", err)
	}
	if after := results(); !slices.Equal(after, both) {
		t.Fatalf("failed run: output directories changed from %v to %v", both, after)
	}
	checkContents(manifest, "tree\r\n")
	for _, dir := range both {
		dataFile := "tree03.in"
		if dir == first[0] {
			dataFile = "sum01.in"
		}
		checkContents(filepath.Join(dir, "测试 数据", dataFile), "1\n")
	}
}
