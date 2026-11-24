package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindInnerArchive(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"a", "b"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, "DATA.7z"), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := findInnerArchive(root, "data.7z"); err == nil {
		t.Fatal("ambiguous basename was accepted")
	}
	got, err := findInnerArchive(root, `B\data.7z`)
	if want := filepath.Join(root, "b", "DATA.7z"); err != nil || got != want {
		t.Fatalf("findInnerArchive() = %q, %v; want %q", got, err, want)
	}
	if _, err := findInnerArchive(root, "missing.zip"); err == nil {
		t.Fatal("missing inner archive was accepted")
	}
}

func TestValidateInnerName(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../p.7z", `C:\p.7z`, `\\host\p.7z`, "a/*", "a:stream", "a//b", "name. "} {
		if err := validateInnerName(name); err == nil {
			t.Errorf("accepted invalid inner name %q", name)
		}
	}
}

// Set CSP_TEST_7Z to an absolute path to the matching native 7-Zip CLI.
func TestExtractEncryptedLayers(t *testing.T) {
	exe := os.Getenv("CSP_TEST_7Z")
	if exe == "" {
		t.Skip("CSP_TEST_7Z is unset")
	}
	base := t.TempDir()
	innerDir := filepath.Join(base, "内层 文件")
	if err := os.Mkdir(innerDir, 0700); err != nil {
		t.Fatal(err)
	}
	const contents = "fixture\n"
	if err := os.WriteFile(filepath.Join(base, "sample.in"), []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "public.txt"), []byte("readable before decryption"), 0600); err != nil {
		t.Fatal(err)
	}
	const innerPW = "Inner %&*- password"
	const outerPW = "🔐外层 -p 密码 "
	inner := filepath.Join(innerDir, "data.zip")
	outer := filepath.Join(base, "outer.7z")
	for _, args := range [][]string{
		{"a", inner, "public.txt"},
		{"a", "-mem=AES256", "-p" + innerPW, inner, "sample.in"},
		{"a", "-mhe=on", "-p" + outerPW, outer, "内层 文件"},
	} {
		cmd := exec.Command(exe, args...)
		cmd.Dir = base
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("create test archive: %v, %s", err, out)
		}
	}
	type answer struct {
		layer    string
		retry    bool
		password string
	}
	for _, tc := range []struct {
		name, innerName, expectedErr string
		answers                      []answer
	}{
		{"success", "DATA.ZIP", "", []answer{{"外层", false, outerPW}, {"内层", false, innerPW}}},
		{"missing inner archive", "absent.zip", "找不到内层压缩包", []answer{{"外层", false, outerPW}}},
		{"retry current layer", "DATA.ZIP", "", []answer{
			{"外层", false, "wrong"}, {"外层", true, outerPW},
			{"内层", false, "wrong"}, {"内层", true, "still wrong"}, {"内层", true, innerPW},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stage := t.TempDir()
			asked := 0
			var previous []byte
			checkCleared := func() {
				if len(bytes.Trim(previous, "\x00")) != 0 {
					t.Fatal("previous password was retained")
				}
			}
			getPassword := func(layer string, retry bool) ([]byte, error) {
				checkCleared()
				if asked >= len(tc.answers) {
					t.Fatalf("unexpected extra prompt for %s (retry=%v)", layer, retry)
				}
				want := tc.answers[asked]
				asked++
				if layer != want.layer || retry != want.retry {
					t.Fatalf("prompt %d = (%s, %v); want (%s, %v)", asked, layer, retry, want.layer, want.retry)
				}
				if layer == "内层" && !retry {
					// The run must finish using the extracted inner archive alone.
					moved := filepath.Join(t.TempDir(), "already-extracted.7z")
					if err := os.Rename(outer, moved); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := os.Rename(moved, outer); err != nil {
							t.Error(err)
						}
					})
				}
				if layer == "内层" && retry {
					// public.txt is readable even with the wrong ZIP password.
					entries, err := os.ReadDir(stage)
					if err != nil || len(entries) != 0 {
						t.Fatalf("failed attempt left partial data: %v, %v", entries, err)
					}
				}
				previous = []byte(want.password)
				return previous, nil
			}
			err := extractLayers(context.Background(), exe, outer, tc.innerName, t.TempDir(), stage, getPassword)
			checkCleared()
			if asked != len(tc.answers) {
				t.Fatalf("got %d prompts, want %d; extraction error: %v", asked, len(tc.answers), err)
			}
			if tc.expectedErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.expectedErr) {
					t.Fatalf("expected error containing %q, got %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(stage, "sample.in"))
			if err != nil || string(data) != contents {
				t.Fatalf("extracted file = %q, %v", data, err)
			}
		})
	}

	t.Run("cancel at retry", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		calls := 0
		ask := func(layer string, retry bool) ([]byte, error) {
			calls++
			if calls > 2 || layer != "外层" || retry != (calls == 2) {
				t.Fatalf("unexpected prompt %d: %s, retry=%v", calls, layer, retry)
			}
			if retry {
				cancel()
				return nil, ctx.Err()
			}
			return []byte("wrong"), nil
		}
		err := extractLayers(ctx, exe, outer, "data.zip", t.TempDir(), t.TempDir(), ask)
		if calls != 2 || !errors.Is(err, context.Canceled) {
			t.Fatalf("got %d prompts, err %v; want cancellation on retry", calls, err)
		}
	})

	t.Run("invalid archive", func(t *testing.T) {
		calls := 0
		ask := func(layer string, retry bool) ([]byte, error) {
			calls++
			if calls != 1 || layer != "外层" || retry {
				t.Fatalf("invalid archive triggered password retry")
			}
			return []byte(outerPW), nil
		}
		err := extractLayers(context.Background(), exe, filepath.Join(base, "public.txt"), "data.zip", t.TempDir(), t.TempDir(), ask)
		if err == nil || calls != 1 {
			t.Fatalf("invalid archive: got %d prompts and err %v", calls, err)
		}
	})
}
