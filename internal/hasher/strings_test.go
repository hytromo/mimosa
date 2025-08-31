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

func TestHexToBytes_EmptyString(t *testing.T) {
	got, err := HexToBytes("")
	if err != nil {
		t.Fatalf("HexToBytes(\"\") returned error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("HexToBytes(\"\") = %v, want empty slice", got)
	}
}

func TestHexToBytes_OddLength(t *testing.T) {
	_, err := HexToBytes("123")
	if err == nil {
		t.Error("HexToBytes(\"123\") did not return error for odd length")
	}
}

func TestHexToBytes_InvalidCharacters(t *testing.T) {
	testCases := []string{
		"gg", // invalid hex character
		"12g3",
		"12G3",
		"12@3",
	}

	for _, tc := range testCases {
		_, err := HexToBytes(tc)
		if err == nil {
			t.Errorf("HexToBytes(%q) did not return error", tc)
		}
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

func TestBytesToHex_EmptySlice(t *testing.T) {
	got := BytesToHex([]byte{})
	if got != "" {
		t.Errorf("BytesToHex([]) = %q, want \"\"", got)
	}
}

func TestBytesToHex_SpecialBytes(t *testing.T) {
	data := []byte{0, 255, 1, 254}
	want := "00ff01fe"
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

func TestBytesToZ85_ErrorCase(t *testing.T) {
	// Z85 encoding requires the input length to be a multiple of 4.
	// This should trigger the error path.
	data := []byte{1, 2, 3} // length 3 is not a multiple of 4
	z85Str := BytesToZ85(data)
	if z85Str != "\x00\x00\x00" {
		t.Errorf("BytesToZ85(%v) = %q, want \"\x00\x00\x00\"", data, z85Str)
	}
}

func TestBytesToZ85_VariousLengths(t *testing.T) {
	testCases := []struct {
		input    []byte
		expected string
	}{
		{[]byte{}, ""},
		{[]byte{1, 2, 3, 4}, "0rJua"},
		{[]byte{1, 2, 3, 4, 5, 6, 7, 8}, "0rJua1Qkhq"},
	}

	for _, tc := range testCases {
		got := BytesToZ85(tc.input)
		if got != tc.expected {
			t.Errorf("BytesToZ85(%v) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestZ85ToBytes_InvalidInputs(t *testing.T) {
	invalidInputs := []string{
		"invalid!!",
		"1234", // not a multiple of 5
	}

	for _, input := range invalidInputs {
		_, err := Z85ToBytes(input)
		if err == nil {
			t.Errorf("Z85ToBytes(%q) did not return error", input)
		}
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

func TestHexToZ85_EmptyString(t *testing.T) {
	got, err := HexToZ85("")
	if err != nil {
		t.Fatalf("HexToZ85(\"\") returned error: %v", err)
	}
	if got != "" {
		t.Errorf("HexToZ85(\"\") = %q, want \"\"", got)
	}
}

func TestHexToZ85_OddLength(t *testing.T) {
	_, err := HexToZ85("123")
	if err == nil {
		t.Error("HexToZ85(\"123\") did not return error for odd length")
	}
}

func TestZ85ToHex_EmptyString(t *testing.T) {
	got, err := Z85ToHex("")
	if err != nil {
		t.Fatalf("Z85ToHex(\"\") returned error: %v", err)
	}
	if got != "" {
		t.Errorf("Z85ToHex(\"\") = %q, want \"\"", got)
	}
}

func TestZ85ToHex_InvalidLength(t *testing.T) {
	_, err := Z85ToHex("1234") // not a multiple of 5
	if err == nil {
		t.Error("Z85ToHex(\"1234\") did not return error for invalid length")
	}
}

func TestRoundTrip_AllFunctions(t *testing.T) {
	// Test complete round-trip through all functions
	originalData := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	// Bytes -> Hex
	hexStr := BytesToHex(originalData)

	// Hex -> Bytes
	bytesFromHex, err := HexToBytes(hexStr)
	if err != nil {
		t.Fatalf("HexToBytes error: %v", err)
	}

	// Bytes -> Z85
	z85Str := BytesToZ85(originalData)

	// Z85 -> Bytes
	bytesFromZ85, err := Z85ToBytes(z85Str)
	if err != nil {
		t.Fatalf("Z85ToBytes error: %v", err)
	}

	// Verify all round-trips work
	if string(bytesFromHex) != string(originalData) {
		t.Errorf("Hex round-trip failed: got %v, want %v", bytesFromHex, originalData)
	}

	if string(bytesFromZ85) != string(originalData) {
		t.Errorf("Z85 round-trip failed: got %v, want %v", bytesFromZ85, originalData)
	}
}
