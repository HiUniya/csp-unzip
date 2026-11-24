package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var errWrongPassword = errors.New("密码错误")

// extractLayers works in caller-owned staging directories. Paths other than
// innerName are absolute, and finalStage must be an empty directory.
func extractLayers(ctx context.Context, sevenZipExe, outerArchive, innerName, scratchDir, finalStage string, getPassword func(layer string, retry bool) ([]byte, error)) error {
	if err := validateInnerName(innerName); err != nil {
		return err
	}
	entries, err := os.ReadDir(finalStage)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("解压暂存目录非空：%s", finalStage)
	}
	outerDir, err := os.MkdirTemp(scratchDir, "outer-")
	if err != nil {
		return fmt.Errorf("创建外层解压目录失败：%w", err)
	}
	extractLayer := func(layer, archive, target string) error {
		for retry := false; ; retry = true {
			if err := ctx.Err(); err != nil {
				return err
			}
			password, err := getPassword(layer, retry)
			if err != nil {
				clear(password)
				return err
			}
			err = extractArchive(ctx, sevenZipExe, archive, target, password)
			clear(password)
			if err == nil {
				return nil
			}
			if !errors.Is(err, errWrongPassword) {
				return fmt.Errorf("%s压缩包 %q 解压失败：%w", layer, filepath.Base(archive), err)
			}
			// A mixed archive may have written some entries before rejecting
			// the password. Reset only this layer's private output directory.
			if err := removeWorkDir(target); err != nil {
				return fmt.Errorf("清理%s解压暂存目录失败：%w", layer, err)
			}
			if err := os.Mkdir(target, 0700); err != nil {
				return fmt.Errorf("重建%s解压暂存目录失败：%w", layer, err)
			}
		}
	}
	if err := extractLayer("外层", outerArchive, outerDir); err != nil {
		return err
	}
	innerArchive, err := findInnerArchive(outerDir, innerName)
	if err != nil {
		return err
	}
	return extractLayer("内层", innerArchive, finalStage)
}

func extractArchive(ctx context.Context, exe, archive, outDir string, password []byte) error {
	if issue := passwordIssue(password); issue != "" {
		return errors.New(issue)
	}
	// 7-Zip reads one UTF-8 line from stdin when encrypted content is opened.
	// The child's working directory contains only our bundled 7-Zip binaries.
	cmd := exec.CommandContext(ctx, exe, "x", "-y", "-sccUTF-8", "-bso0", "-bsp0",
		"-snl-", "-snh-", "-sns-", "-spd", "-o"+outDir, "--", archive)
	cmd.Dir = filepath.Dir(exe)
	cmd.Stdin = io.MultiReader(bytes.NewReader(password), strings.NewReader("\n"))
	_, err := cmd.Output()
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		detail := strings.TrimSpace(string(exit.Stderr))
		if exit.ExitCode() == 2 && isPasswordDiagnostic(detail) {
			return errWrongPassword
		}
		return fmt.Errorf("7-Zip 返回代码 %d，请检查密码、压缩包及可用磁盘空间。%s", exit.ExitCode(), detail)
	}
	return fmt.Errorf("无法运行 7-Zip：%w", err)
}

func validateInnerName(name string) error {
	slashName := strings.ReplaceAll(name, `\`, "/")
	if slashName == "." || !fs.ValidPath(slashName) ||
		!filepath.IsLocal(filepath.FromSlash(slashName)) || strings.ContainsAny(slashName, ":*?\"<>|") {
		return fmt.Errorf("无效的内层压缩包名 %q，请填写文件名或包内相对路径", name)
	}
	for _, part := range strings.Split(slashName, "/") {
		if strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") ||
			strings.IndexFunc(part, func(r rune) bool { return r < 32 }) != -1 {
			return fmt.Errorf("内层压缩包名包含无效 Windows 路径 %q", part)
		}
	}
	return nil
}

func findInnerArchive(root, name string) (string, error) {
	expected := strings.ReplaceAll(name, `\`, "/")
	exactPath := strings.Contains(expected, "/")
	var found string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("外层解压目录中含有链接或特殊文件：%s", path)
		}
		candidate := entry.Name()
		if exactPath {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			candidate = filepath.ToSlash(rel)
		}
		if strings.EqualFold(candidate, expected) {
			if found != "" {
				return fmt.Errorf("外层包中有多个匹配 %q 的文件，请在配置中填写唯一的包内相对路径", name)
			}
			found = path
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("外层解压结果中找不到内层压缩包 %q，请核对配置", name)
	}
	return found, nil
}
