package main

import (
	"encoding/json"
	"os"
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
	jsonData, _ := json.Marshal(target)
	err := os.WriteFile(path, jsonData, 0600) // #nosec inside container
	return err
}
