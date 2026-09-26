package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestCollectGoFilesDirectory(t *testing.T) {
	got, err := collectGoFiles("testdata/collect")
	if err != nil {
		t.Fatalf("collectGoFiles: %v", err)
	}
	sort.Strings(got)

	want := []string{
		filepath.Join("testdata/collect", "root.go"),
		filepath.Join("testdata/collect", "sub", "nested.go"),
	}
	if len(got) != len(want) {
		t.Fatalf("collectGoFiles() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("collectGoFiles()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCollectGoFilesSinglePath(t *testing.T) {
	got, err := collectGoFiles("testdata/collect/root.go")
	if err != nil {
		t.Fatalf("collectGoFiles: %v", err)
	}
	if len(got) != 1 || got[0] != "testdata/collect/root.go" {
		t.Fatalf("collectGoFiles() = %v, want [testdata/collect/root.go]", got)
	}
}

func TestCollectGoFilesSkipsNestedTestdata(t *testing.T) {
	got, err := collectGoFiles("testdata/skipcheck")
	if err != nil {
		t.Fatalf("collectGoFiles: %v", err)
	}
	if len(got) != 1 || got[0] != "testdata/skipcheck/plain.go" {
		t.Fatalf("collectGoFiles() = %v, want [testdata/skipcheck/plain.go]", got)
	}
}

func TestCollectGoFilesNonGoFile(t *testing.T) {
	got, err := collectGoFiles("testdata/collect/notes.txt")
	if err != nil {
		t.Fatalf("collectGoFiles: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("collectGoFiles() = %v, want none", got)
	}
}

func TestPrintFindingsJSON(t *testing.T) {
	findings := []Finding{
		{File: "routes.go", Line: 12, Message: "duplicate route"},
	}

	out := captureStdout(t, func() {
		printFindingsJSON(findings)
	})

	var got []Finding
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}
	if len(got) != 1 || got[0] != findings[0] {
		t.Fatalf("printFindingsJSON output = %+v, want %+v", got, findings)
	}
}

func TestPrintFindingsJSONEmpty(t *testing.T) {
	out := captureStdout(t, func() {
		printFindingsJSON(nil)
	})

	var got []Finding
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("printFindingsJSON(nil) = %v, want an empty array, not null", out)
	}
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns
// everything fn wrote to it.
func captureStdout(t *testing.T, fn func()) []byte {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}
	return buf.Bytes()
}
