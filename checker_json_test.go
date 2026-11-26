package main

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestUpdateCheckerProblemsChangesOnlyTopLevelArray(t *testing.T) {
	const before = "\ufeff{\r\n\t\"other\":{\"problems\":[42],\"n\":1.234567890123456789e+11},\r\n\t\"pr\\u006fblems\" \t: \t"
	const after = " \t,\r\n\t\"problem_extern\":false,\r\n\t\"problem_regex\":\"^{0}\\\\\\\\{0}\\\\.(cpp|c|pas)$\"\r\n}\r\n"
	for _, old := range []string{"[]", `[{"name":"old","regex":"escaped \" quote and ]"}]`} {
		got, err := updateCheckerProblems([]byte(before+old+after), []string{"candy", "foo.bar"})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(got, []byte(before)) || !bytes.HasSuffix(got, []byte(after)) {
			t.Fatalf("changed bytes outside problems: %s", got)
		}
		array := got[len(before) : len(got)-len(after)]
		if bytes.Contains(bytes.ReplaceAll(array, []byte("\r\n"), nil), []byte("\n")) {
			t.Fatalf("inserted bare LF: %q", array)
		}
		var entries []checkerProblem
		if err := json.Unmarshal(array, &entries); err != nil || len(entries) != 2 {
			t.Fatalf("entries %q: %v", array, err)
		}
		if entries[0].Name != "candy" || entries[0].Regex != `^candy\\candy\.(cpp|c|pas)$` ||
			entries[1].Name != "foo.bar" || !regexp.MustCompile(entries[1].Regex).MatchString(`foo.bar\foo.bar.pas`) ||
			regexp.MustCompile(entries[1].Regex).MatchString(`fooXbar\fooXbar.cpp`) {
			t.Fatalf("unexpected entries: %+v", entries)
		}
	}
}

func TestCheckerProblemRegex(t *testing.T) {
	for _, tc := range []struct{ cfg, want string }{
		{`{"problems":[]}`, `^test\\test\.(cpp|c|pas)$`},
		{`{"problem_regex":"^{0}/{0}\\.py$","problems":[]}`, `^test/test\.py$`},
	} {
		got, err := updateCheckerProblems([]byte(tc.cfg), []string{"test"})
		var cfg struct{ Problems []checkerProblem }
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(got, &cfg); err != nil || len(cfg.Problems) != 1 || cfg.Problems[0].Regex != tc.want || bytes.ContainsRune(got, '\n') {
			t.Fatalf("updated compact config %s: %v", got, err)
		}
	}
}

func TestInvalidCheckerProblems(t *testing.T) {
	for _, cfg := range []string{
		"[]", "null", `{"x":{"problems":[]}}`, `{"problems":null}`, `{"problems":"[]"}`,
		`{"problems":[],"problems":[]}`, `{"problems":[],"problem_regex":null}`,
		`{"problems":[],"problem_regex":"x","problem_regex":"y"}`,
		`{"problems":[]}{"second":true}`, `{"problems":[],}`, "{\"problems\":[], \"bad\":\"\xff\"}",
	} {
		if got, err := updateCheckerProblems([]byte(cfg), []string{"x"}); err == nil || got != nil {
			t.Errorf("accepted invalid config %q", cfg)
		}
	}
	got, err := updateCheckerProblems([]byte(`{"problems":[{"name":"old"}]}`), nil)
	if err != nil || strings.TrimSpace(string(got)) != `{"problems":[]}` {
		t.Fatalf("empty list = %s; %v", got, err)
	}
}
