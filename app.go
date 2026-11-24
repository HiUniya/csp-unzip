package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func run(ctx context.Context, base string, ask passwordPrompt, out io.Writer, install7Zip func(string) (string, error)) error {
	c, err := loadConfig(base)
	if err != nil {
		return err
	}
	outer := filepath.Join(base, c.outer)
	if strings.EqualFold(c.outer, problemListName) {
		return fmt.Errorf("外层压缩包与题名清单同名，请重命名压缩包并更新 %s", configName)
	}
	info, err := os.Stat(outer)
	if err != nil {
		return fmt.Errorf("读取外层压缩包失败：%w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("外层压缩包应为普通文件：%s", outer)
	}
	fmt.Fprintf(out, "外层包：%s\n内层包：%s\n\n", c.outer, c.inner)
	// Staging beside the destination allows publishing the complete tree with one rename.
	work, err := os.MkdirTemp(base, ".csp-unzip-")
	if err != nil {
		return fmt.Errorf("无法在程序目录创建暂存目录：%w", err)
	}
	defer func() {
		if err := removeWorkDir(work); err != nil {
			fmt.Fprintf(out, "临时目录清理失败：%s（%v）\n", work, err)
		}
	}()
	exe, err := install7Zip(work)
	if err != nil {
		return fmt.Errorf("释放内置 7-Zip 失败：%w", err)
	}
	stage := filepath.Join(work, "result")
	if err := os.Mkdir(stage, 0700); err != nil {
		return err
	}
	getPassword := func(layer string, retry bool) ([]byte, error) {
		if retry {
			fmt.Fprintln(out, layer+"密码错误，请重新输入。")
		}
		password, err := promptLayerPassword(layer, ask, out)
		if err == nil {
			fmt.Fprintln(out, "正在解开"+layer+"压缩包……")
		}
		return password, err
	}
	if err := extractLayers(ctx, exe, outer, c.inner, work, stage, getPassword); err != nil {
		return err
	}
	fmt.Fprintln(out, "正在扫描测试数据……")
	summary, err := scanProblems(stage)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	dest, err := publishResult(base, stage, summary.Problems, time.Now())
	if err != nil {
		return err
	}
	for _, skipped := range summary.Skipped {
		fmt.Fprintf(out, "未识别出英文题名，已跳过：%s\n", skipped)
	}
	fmt.Fprintf(out, "完成。共扫描 %d 个 .in 文件，其中 %d 个在同目录有同名 .ans，生成 %d 个题名。\n解压目录：%s\n清单：%s\n",
		summary.Inputs, summary.Paired, len(summary.Problems), dest, filepath.Join(base, problemListName))
	return nil
}

func removeWorkDir(path string) error {
	if err := os.RemoveAll(path); err == nil {
		return nil
	}
	// Archives can mark their contents read-only on Windows.
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Type().IsRegular() {
			return os.Chmod(p, 0700)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return os.RemoveAll(path)
}
