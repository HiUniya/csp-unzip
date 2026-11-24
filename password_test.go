package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestPasswordPunctuation(t *testing.T) {
	for _, r := range "，。、；：？！“”‘’【】《》（）……——＋￥～＠·「」﹁︿　" {
		if msg := passwordIssue([]byte("abc" + string(r))); !strings.Contains(msg, "切换英文输入法") {
			t.Errorf("character %q did not trigger the input method hint", r)
		}
	}
	for _, valid := range []string{" 外层密码〇🔐 ", "abc.,;:'?!()[]<>%+&$@#~", "Ｆｏｏ０１２"} {
		if msg := passwordIssue([]byte(valid)); msg != "" {
			t.Errorf("unexpected validation error for %q: %s", valid, msg)
		}
	}
}

func TestPromptLayerPassword(t *testing.T) {
	for _, tc := range []struct {
		name     string
		inputs   []string
		wantHint bool
	}{
		{"single entry", []string{"pass!"}, false},
		{"retry Chinese punctuation", []string{"pass！", "pass!"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			inputs := tc.inputs
			got, err := promptLayerPassword("内层", func(string) ([]byte, error) {
				if len(inputs) == 0 {
					return nil, io.EOF
				}
				answer := inputs[0]
				inputs = inputs[1:]
				return []byte(answer), nil
			}, &out)
			if err != nil || len(inputs) != 0 || string(got) != "pass!" || strings.Contains(out.String(), "切换英文输入法") != tc.wantHint {
				t.Fatalf("password = %q, error = %v, prompt = %q", got, err, out.String())
			}
		})
	}
}
