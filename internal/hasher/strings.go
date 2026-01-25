package hasher

import (
	"encoding/hex"

	"github.com/kalafut/imohash"
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
