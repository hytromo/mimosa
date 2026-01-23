package hasher

import (
	"encoding/hex"
	"testing"

	"github.com/kalafut/imohash"
)

func TestHashStrings(t *testing.T) {
	// Empty input
	if got := HashStrings([]string{}); got != "" {
		t.Errorf("HashStrings([]) = %q, want \"\"", got)
	}

	// Single string
	s := "hello"
	imohashV := imohash.Sum([]byte(s))
	want := hex.EncodeToString(imohashV[:])
	if got := HashStrings([]string{s}); got != want {
		t.Errorf("HashStrings([%q]) = %q, want %q", s, got, want)
	}

	// Multiple strings
	ss := []string{"foo", "bar", "baz"}
	concat := "foobarbaz"
	imohashV = imohash.Sum([]byte(concat))
	want = hex.EncodeToString(imohashV[:])
	if got := HashStrings(ss); got != want {
		t.Errorf("HashStrings(%v) = %q, want %q", ss, got, want)
	}
}

func TestHashStrings_EmptyStrings(t *testing.T) {
	// Test with empty strings
	ss := []string{"", "hello", ""}
	concat := "hello"
	imohashV := imohash.Sum([]byte(concat))
	want := hex.EncodeToString(imohashV[:])
	if got := HashStrings(ss); got != want {
		t.Errorf("HashStrings(%v) = %q, want %q", ss, got, want)
	}
}

func TestHashStrings_LargeStrings(t *testing.T) {
	// Test with large strings
	largeStr := "a"
	for i := 0; i < 1000; i++ {
		largeStr += "a"
	}
	ss := []string{largeStr, "b", largeStr}
	concat := largeStr + "b" + largeStr
	imohashV := imohash.Sum([]byte(concat))
	want := hex.EncodeToString(imohashV[:])
	if got := HashStrings(ss); got != want {
		t.Errorf("HashStrings with large strings = %q, want %q", got, want)
	}
}

func TestHashStrings_SpecialCharacters(t *testing.T) {
	// Test with special characters
	ss := []string{"hello\nworld", "tab\there", "quote\"test"}
	concat := "hello\nworld" + "tab\there" + "quote\"test"
	imohashV := imohash.Sum([]byte(concat))
	want := hex.EncodeToString(imohashV[:])
	if got := HashStrings(ss); got != want {
		t.Errorf("HashStrings with special chars = %q, want %q", got, want)
	}
}

func TestHashStrings_UnicodeStrings(t *testing.T) {
	// Test with unicode strings
	ss := []string{"hello", "世界", "привет", "مرحبا"}
	concat := "hello" + "世界" + "привет" + "مرحبا"
	imohashV := imohash.Sum([]byte(concat))
	want := hex.EncodeToString(imohashV[:])
	if got := HashStrings(ss); got != want {
		t.Errorf("HashStrings with unicode = %q, want %q", got, want)
	}
}
