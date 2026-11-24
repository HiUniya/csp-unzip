package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

const problemListName = "problem.extern"

type scanSummary struct {
	Problems []string
	Inputs   int
	Paired   int
	Skipped  []string
}

func scanProblems(root string) (scanSummary, error) {
	var summary scanSummary
	var inputs []string
	answers := make(map[string]bool)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".in" && ext != ".ans" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if ext == ".in" {
			inputs = append(inputs, rel)
		} else {
			answers[strings.ToLower(rel[:len(rel)-len(ext)])] = true
		}
		return nil
	})
	if err != nil {
		return summary, err
	}

	summary.Inputs = len(inputs)
	spelling := make(map[string]string)
	for _, input := range inputs {
		stemPath := strings.TrimSuffix(input, filepath.Ext(input))
		if answers[strings.ToLower(stemPath)] {
			summary.Paired++
		}
		stem := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
		name := strings.Trim(caseDigits.ReplaceAllString(stem, ""), "._ -")
		if !englishName.MatchString(name) {
			summary.Skipped = append(summary.Skipped, input)
			continue
		}
		folded := strings.ToLower(name)
		if previous, exists := spelling[folded]; exists && previous != name {
			spelling[folded] = folded
		} else {
			spelling[folded] = name
		}
	}
	if len(inputs) == 0 {
		return summary, fmt.Errorf("最终解压目录中没有找到 .in 文件")
	}
	if len(spelling) == 0 {
		return summary, fmt.Errorf("%d 个 .in 文件均无法识别英文题名（例如 %q），文件名示例为 sum01.in", summary.Inputs, summary.Skipped[0])
	}

	for _, name := range spelling {
		summary.Problems = append(summary.Problems, name)
	}
	sort.Slice(summary.Problems, func(i, j int) bool {
		return strings.ToLower(summary.Problems[i]) < strings.ToLower(summary.Problems[j])
	})
	return summary, nil
}

var caseDigits = regexp.MustCompile(`[0-9]+`)
var englishName = regexp.MustCompile(`^[A-Za-z]+(?:[._ -]+[A-Za-z]+)*$`)

// stage is inside this run's private work directory on the destination volume.
func publishResult(base, stage string, names []string, stamp time.Time) (string, error) {
	pending := filepath.Join(filepath.Dir(stage), problemListName)
	content := strings.Join(names, "\r\n") + "\r\n"
	if err := os.WriteFile(pending, []byte(content), 0600); err != nil {
		return "", fmt.Errorf("准备题名清单失败：%w", err)
	}
	dest, err := moveToDatedDirectory(stage, base, stamp)
	if err != nil {
		return "", fmt.Errorf("保存解压目录失败：%w", err)
	}
	if err := replaceProblemList(pending, filepath.Join(base, problemListName)); err != nil {
		// The previous manifest remains in place. Withdraw only this run's data.
		if undoErr := os.Rename(dest, stage); undoErr != nil {
			return "", fmt.Errorf("更新题名清单失败：%w。解压文件留在 %s（撤回目录失败：%v）", err, dest, undoErr)
		}
		return "", fmt.Errorf("更新题名清单失败：%w", err)
	}
	return dest, nil
}

func moveToDatedDirectory(stage, base string, stamp time.Time) (string, error) {
	first := "down" + stamp.Format("20060102_150405")
	for n := 1; ; n++ {
		name := first
		if n > 1 {
			name = fmt.Sprintf("%s_%d", first, n)
		}
		dest := filepath.Join(base, name)
		if _, err := os.Lstat(dest); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return "", err
		}
		if err := os.Rename(stage, dest); err != nil {
			// Another extraction can publish its nonempty directory between
			// Lstat and Rename. Choose a fresh suffix if it won the name.
			if _, statErr := os.Lstat(dest); statErr == nil {
				continue
			}
			return "", err
		}
		return dest, nil
	}
}

func replaceProblemList(pending, dest string) error {
	err := os.Rename(pending, dest)
	if err == nil || runtime.GOOS != "windows" {
		return err
	}
	// Windows disallows replacing a read-only file.
	info, statErr := os.Lstat(dest)
	if statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0200 != 0 {
		return err
	}
	if err := os.Chmod(dest, info.Mode().Perm()|0200); err != nil {
		return err
	}
	if err := os.Rename(pending, dest); err != nil {
		if current, statErr := os.Lstat(dest); statErr == nil && os.SameFile(info, current) {
			if modeErr := os.Chmod(dest, info.Mode().Perm()); modeErr != nil {
				return fmt.Errorf("%w（恢复原清单的只读属性失败：%v）", err, modeErr)
			}
		}
		return err
	}
	return nil
}
