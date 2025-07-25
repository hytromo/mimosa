package hasher

import (
	"encoding/hex"
	"fmt"

	"github.com/kalafut/imohash"
	"github.com/tilinna/z85"
)

func HashStrings(toHash []string) string {
	if len(toHash) == 0 {
		return ""
	}

	var bigString string
	for _, s := range toHash {
		bigString += s
	}

	h := imohash.Sum([]byte(bigString))
	return hex.EncodeToString(h[:])
}

func HexToBytes(hexStr string) ([]byte, error) {
	decoded, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex string: %w", err)
	}
	return decoded, nil
}

func BytesToZ85(data []byte) string {
	encoded := make([]byte, z85.EncodedLen(len(data)))
	z85.Encode(encoded, data)
	return string(encoded)
}

func HexToZ85(hexStr string) (string, error) {
	bytes, err := HexToBytes(hexStr)
	if err != nil {
		return "", fmt.Errorf("failed to convert hex to bytes: %w", err)
	}

	z85Str := BytesToZ85(bytes)
	return z85Str, nil
}

func Z85ToHex(z85Str string) (string, error) {
	bytes, err := Z85ToBytes(z85Str)
	if err != nil {
		return "", fmt.Errorf("failed to convert Z85 to bytes: %w", err)
	}
	hexStr := BytesToHex(bytes)
	return hexStr, nil
}

func BytesToHex(data []byte) string {
	return hex.EncodeToString(data)
}

func Z85ToBytes(z85Str string) ([]byte, error) {
	decoded := make([]byte, z85.DecodedLen(len(z85Str)))

	_, err := z85.Decode(decoded, []byte(z85Str))
	if err != nil {
		return nil, fmt.Errorf("failed to decode Z85 string: %w", err)
	}

	return decoded, nil
}
