package fileutil

import (
	"encoding/json"
	"os"
)

// SaveJSON saves a struct as pretty-formatted JSON data to a specific path
func SaveJSON(path string, dataToWrite interface{}) error {
	bytesToWrite, err := json.MarshalIndent(dataToWrite, "", "\t")

	if err != nil {
		return err
	}

	return os.WriteFile(path, bytesToWrite, 0644)
}
