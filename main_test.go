package main

import (
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
