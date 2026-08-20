package commandid

import (
	"path/filepath"
	"testing"
)

func TestIdentityNormalizesProjectRoot(t *testing.T) {
	root := t.TempDir()
	first, normalizedFirst, err := Identity(root, []string{"go", "test", "./..."})
	if err != nil {
		t.Fatal(err)
	}
	second, normalizedSecond, err := Identity(filepath.Join(root, "."), []string{"go", "test", "./..."})
	if err != nil {
		t.Fatal(err)
	}
	if first != second || normalizedFirst != normalizedSecond {
		t.Fatalf("equivalent roots differ: %q/%q vs %q/%q", first, normalizedFirst, second, normalizedSecond)
	}
	if len(first) < 9 || first[:8] != "go-test-" {
		t.Fatalf("unexpected human-readable ID %q", first)
	}
}

func TestIdentityPreservesArgumentOrdering(t *testing.T) {
	root := t.TempDir()
	first, _, err := Identity(root, []string{"tool", "--include", "a", "--exclude", "b"})
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := Identity(root, []string{"tool", "--exclude", "b", "--include", "a"})
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("argument reordering produced the same ID %q", first)
	}
}

func TestIdentityIsStable(t *testing.T) {
	root := t.TempDir()
	want, _, _ := Identity(root, []string{"npm", "test"})
	for index := 0; index < 20; index++ {
		got, _, err := Identity(root, []string{"npm", "test"})
		if err != nil || got != want {
			t.Fatalf("iteration %d: got %q err=%v, want %q", index, got, err, want)
		}
	}
}
