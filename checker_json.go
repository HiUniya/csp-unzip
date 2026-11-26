package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

type checkerProblem struct {
	Name  string `json:"name"`
	Regex string `json:"regex"`
}

func updateCheckerProblems(data []byte, names []string) ([]byte, error) {
	body := bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if !utf8.Valid(body) || !json.Valid(body) {
		return nil, fmt.Errorf("配置文件不是有效的 UTF-8 JSON")
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	root, err := dec.Token()
	if err != nil || root != json.Delim('{') {
		return nil, fmt.Errorf("配置文件的顶层应为 JSON 对象")
	}

	pattern := `^{0}\\{0}\.(cpp|c|pas)$`
	var havePattern bool
	var start, end, keyEnd int
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil, err
		}
		afterKey := int(dec.InputOffset())
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		switch key {
		case "problems":
			if end != 0 || raw[0] != '[' {
				return nil, fmt.Errorf("顶层 problems 必须是唯一的数组")
			}
			// RawMessage is the exact value, excluding whitespace around it.
			end = int(dec.InputOffset())
			start, keyEnd = end-len(raw), afterKey
		case "problem_regex":
			if havePattern || raw[0] != '"' {
				return nil, fmt.Errorf("顶层 problem_regex 必须是唯一的字符串")
			}
			if err := json.Unmarshal(raw, &pattern); err != nil {
				return nil, err
			}
			havePattern = true
		}
	}
	if end == 0 {
		return nil, fmt.Errorf("缺少顶层 problems 数组")
	}

	problems := make([]checkerProblem, len(names))
	for i, name := range names {
		problems[i] = checkerProblem{Name: name, Regex: strings.ReplaceAll(pattern, "{0}", regexp.QuoteMeta(name))}
	}
	array, err := marshalCheckerProblems(problems, body, keyEnd)
	if err != nil {
		return nil, err
	}
	start += len(data) - len(body)
	end += len(data) - len(body)
	out := make([]byte, 0, len(data)-(end-start)+len(array))
	out = append(out, data[:start]...)
	out = append(out, array...)
	return append(out, data[end:]...), nil
}

func marshalCheckerProblems(problems []checkerProblem, body []byte, keyEnd int) ([]byte, error) {
	if !bytes.Contains(bytes.TrimSpace(body), []byte("\n")) {
		return json.Marshal(problems)
	}
	lineStart := bytes.LastIndexByte(body[:keyEnd], '\n') + 1
	pos := lineStart
	for pos < keyEnd && (body[pos] == ' ' || body[pos] == '\t') {
		pos++
	}
	prefix := string(body[lineStart:pos])
	indent := "  "
	if strings.HasSuffix(prefix, "\t") {
		indent = "\t"
	}
	array, err := json.MarshalIndent(problems, prefix, indent)
	if bytes.Contains(body, []byte("\r\n")) {
		array = bytes.ReplaceAll(array, []byte("\n"), []byte("\r\n"))
	}
	return array, err
}
