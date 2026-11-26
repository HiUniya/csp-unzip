package main

import (
	"context"
	"errors"
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
	checkerPath := filepath.Join(base, checkerConfigName)
	if strings.EqualFold(c.outer, checkerConfigName) {
		return fmt.Errorf("外层压缩包与检查器配置同名，请重命名压缩包并更新 %s", configName)
	}
	info, err := os.Stat(outer)
	if err != nil {
		return fmt.Errorf("读取外层压缩包失败：%w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("外层压缩包应为普通文件：%s", outer)
	}
	if _, err := checkerFileInfo(checkerPath); err != nil {
		return err
	}
	fmt.Fprintf(out, "外层包：%s\n内层包：%s\n\n", c.outer, c.inner)
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
	var dest string
	var summary scanSummary
	err = withWorkDir(base, func(work string) error {
		exe, err := install7Zip(work)
		if err != nil {
			return fmt.Errorf("释放内置 7-Zip 失败：%w", err)
		}
		stage := filepath.Join(work, "result")
		if err := os.Mkdir(stage, 0700); err != nil {
			return err
		}
		if err := extractLayers(ctx, exe, outer, c.inner, work, stage, getPassword); err != nil {
			return err
		}
		fmt.Fprintln(out, "正在扫描测试数据……")
		summary, err = scanProblems(stage)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		dest, err = publishResult(base, stage, summary.Problems, time.Now())
		return err
	})
	if err != nil {
		if dest != "" {
			return fmt.Errorf("解压目录已保存到 %s，已更新 %s 的 problems。\n%w", dest, checkerPath, err)
		}
		return err
	}
	for _, skipped := range summary.Skipped {
		fmt.Fprintf(out, "未识别出英文题名，已跳过：%s\n", skipped)
	}
	fmt.Fprintf(out, "完成。共扫描 %d 个 .in 文件，其中 %d 个在同目录有同名 .ans，已将 %d 道题写入 problems。\n解压目录：%s\n检查器配置：%s\n",
		summary.Inputs, summary.Paired, len(summary.Problems), dest, checkerPath)
	return nil
}

// Stage on the destination volume so the completed tree can be renamed into place.
func withWorkDir(base string, use func(string) error) (err error) {
	work, err := os.MkdirTemp(base, ".csp-unzip-")
	if err != nil {
		return fmt.Errorf("无法在程序目录创建暂存目录：%w", err)
	}
	defer func() {
		if cleanupErr := removeWorkDir(work); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("临时目录未能删除：%s（%w）。关闭占用它的程序后可手动删除此目录", work, cleanupErr))
		}
	}()
	return use(work)
}

func removeWorkDir(path string) error {
	return retryFileOperation(func() error { return tryRemoveWorkDir(path) })
}

func retryFileOperation(operation func() error) error {
	err := operation()
	for _, delayMS := range [...]int{50, 100, 200, 400, 800, 1000} {
		if !retryableCleanupError(err) {
			break
		}
		time.Sleep(time.Duration(delayMS) * time.Millisecond)
		err = operation()
	}
	return err
}

func tryRemoveWorkDir(path string) error {
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
