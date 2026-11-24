package main

import (
	"bytes"
	"fmt"
	"io"
	"unicode"
)

type passwordPrompt func(string) ([]byte, error)

func promptLayerPassword(layer string, ask passwordPrompt, out io.Writer) ([]byte, error) {
	for {
		password, err := ask("请输入" + layer + "密码")
		if err != nil {
			return nil, err
		}
		if issue := passwordIssue(password); issue != "" {
			clear(password)
			fmt.Fprintln(out, issue)
			continue
		}
		return password, nil
	}
}

func passwordIssue(password []byte) string {
	if len(password) == 0 || bytes.IndexAny(password, "\x00\r\n") >= 0 {
		return "密码不能为空，也不能包含换行或空字符。"
	}
	if bytes.IndexFunc(password, isChinesePunctuation) >= 0 {
		return "密码中含有中文标点或全角符号。请切换英文输入法，重新输入该层密码。"
	}
	return ""
}

func isChinesePunctuation(r rune) bool {
	switch r {
	case '‘', '’', '“', '”', '…', '—', '·', '\u3000':
		return true
	}
	// Chinese input methods use CJK, vertical, small and fullwidth forms.
	switch {
	case r >= 0x3000 && r <= 0x303f,
		r >= 0xfe10 && r <= 0xfe19,
		r >= 0xfe30 && r <= 0xfe6f,
		r >= 0xff01 && r <= 0xff65,
		r >= 0xffe0 && r <= 0xffe6:
		return unicode.IsPunct(r) || unicode.IsSymbol(r)
	}
	return false
}
