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

func TestHexToBytes(t *testing.T) {
	// Valid hex
	hexStr := "68656c6c6f"
	want := []byte("hello")
	got, err := HexToBytes(hexStr)
	if err != nil {
		t.Fatalf("HexToBytes(%q) returned error: %v", hexStr, err)
	}
	if string(got) != string(want) {
		t.Errorf("HexToBytes(%q) = %v, want %v", hexStr, got, want)
	}

	// Invalid hex
	_, err = HexToBytes("zzzz")
	if err == nil {
		t.Error("HexToBytes(\"zzzz\") did not return error")
	}
}

func TestBytesToHex(t *testing.T) {
	data := []byte("hello")
	want := "68656c6c6f"
	got := BytesToHex(data)
	if got != want {
		t.Errorf("BytesToHex(%v) = %q, want %q", data, got, want)
	}
}

func TestBytesToZ85AndZ85ToBytes(t *testing.T) {
	// Test round-trip
	data := []byte("40e5e9770c7b55975d42933c56dc6a9f")
	z85Str := BytesToZ85(data)
	got, err := Z85ToBytes(z85Str)
	if err != nil {
		t.Fatalf("Z85ToBytes(%q) error: %v", z85Str, err)
	}
	if string(got) != string(data) {
		t.Errorf("Z85ToBytes(BytesToZ85(%v)) = %v, want %v", data, got, data)
	}

	// Test empty slice
	z85Str = BytesToZ85([]byte{})
	got, err = Z85ToBytes(z85Str)
	if err != nil {
		t.Fatalf("Z85ToBytes(empty) error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Z85ToBytes(BytesToZ85([])) = %v, want []", got)
	}

	// Test invalid Z85 string
	_, err = Z85ToBytes("invalid!!")
	if err == nil {
		t.Error("Z85ToBytes(\"invalid!!\") did not return error")
	}
}

func TestHexToZ85AndZ85ToHex(t *testing.T) {
	// Valid round-trip
	hexStr := "00112233445566778899aabbccddeeff"
	z85Str, err := HexToZ85(hexStr)
	if err != nil {
		t.Fatalf("HexToZ85(%q) error: %v", hexStr, err)
	}
	gotHex, err := Z85ToHex(z85Str)
	if err != nil {
		t.Fatalf("Z85ToHex(%q) error: %v", z85Str, err)
	}
	if gotHex != hexStr {
		t.Errorf("Z85ToHex(HexToZ85(%q)) = %q, want %q", hexStr, gotHex, hexStr)
	}

	// Invalid hex input
	_, err = HexToZ85("nothex")
	if err == nil {
		t.Error("HexToZ85(\"nothex\") did not return error")
	}

	// Invalid Z85 input
	_, err = Z85ToHex("invalid!!")
	if err == nil {
		t.Error("Z85ToHex(\"invalid!!\") did not return error")
	}
}

func TestBytesToZ85_ErrorCase(t *testing.T) {
	// Z85 encoding requires the input length to be a multiple of 4.
	// This should trigger the error path.
	data := []byte{1, 2, 3} // length 3 is not a multiple of 4
	z85Str := BytesToZ85(data)
	if z85Str != "\x00\x00\x00" {
		t.Errorf("BytesToZ85(%v) = %q, want \"\x00\x00\x00\"", data, z85Str)
	}
}
