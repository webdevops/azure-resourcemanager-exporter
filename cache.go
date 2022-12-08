package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func cacheRestoreFromPath(path string, target interface{}) error {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		jsonContent, _ := os.ReadFile(path) // #nosec inside container
		err := json.Unmarshal(jsonContent, &target)
		if err != nil {
			return err
		}
	}

	return nil
}

func cacheSaveToPath(path string, target interface{}) error {
	tmpFilePath := filepath.Join(
		filepath.Dir(path),
		fmt.Sprintf(
			".%s.tmp",
			filepath.Base(path),
		),
	)

	jsonData, _ := json.Marshal(target)
	err := os.WriteFile(tmpFilePath, jsonData, 0600) // #nosec inside container
	if err != nil {
		return err
	}

	// rename file to final cache file (atomic operation)
	err = os.Rename(tmpFilePath, path)
	if err != nil {
		return err
	}

	return nil
}
