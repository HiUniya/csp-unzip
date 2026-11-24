package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const configName = "csp-unzip.cfg"

type config struct {
	outer string
	inner string
}

func loadConfig(base string) (config, error) {
	var c config
	data, err := os.ReadFile(filepath.Join(base, configName))
	if err != nil {
		return c, fmt.Errorf("读取同目录的 %s 失败：%w", configName, err)
	}
	if !utf8.Valid(data) {
		return c, fmt.Errorf("请将 %s 保存为 UTF-8 编码", configName)
	}
	fields := map[string]*string{"outer_archive": &c.outer, "inner_archive": &c.inner}
	for i, raw := range strings.Split(strings.TrimPrefix(string(data), "\ufeff"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if ok && key == "output_dir" {
			// Older configurations supplied a fixed output directory.
			continue
		}
		field := fields[key]
		if !ok || field == nil || *field != "" || value == "" {
			return c, fmt.Errorf("%s 第 %d 行配置无效，请检查键名、重复项和空值", configName, i+1)
		}
		*field = value
	}
	for _, key := range []string{"outer_archive", "inner_archive"} {
		value := *fields[key]
		if value == "" {
			return c, fmt.Errorf("%s 缺少 %s", configName, key)
		}
		if key != "inner_archive" && strings.ContainsAny(value, "/\\") {
			return c, fmt.Errorf("%s 应为程序同目录下的名称：%q", key, value)
		}
		if err := validateInnerName(value); err != nil {
			return c, fmt.Errorf("%s 路径无效：%q", key, value)
		}
	}
	return c, nil
}
