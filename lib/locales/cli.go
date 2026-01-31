package locales

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func Handle() {

	dir1 := os.Args[2]
	dir2 := os.Args[3]

	adminPadsDir := filepath.Join(dir2, "ep_admin_pads")

	err := filepath.WalkDir(dir1, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}

		sourceFile := path
		targetFile := filepath.Join(adminPadsDir, d.Name())

		if _, err := os.Stat(targetFile); err != nil {
			return nil
		}

		fmt.Printf("Verarbeite: %s -> %s\n", sourceFile, targetFile)

		return copyAdminKeys(sourceFile, targetFile)
	})

	if err != nil {
		fmt.Printf("Fehler beim Verarbeiten der Verzeichnisse: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

func copyAdminKeys(sourcePath, targetPath string) error {
	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}

	targetData, err := os.ReadFile(targetPath)
	if err != nil {
		return err
	}

	sourceJSON := make(map[string]interface{})
	targetJSON := make(map[string]interface{})

	if err := json.Unmarshal(sourceData, &sourceJSON); err != nil {
		return err
	}

	if err := json.Unmarshal(targetData, &targetJSON); err != nil {
		return err
	}

	for key, value := range sourceJSON {
		if strings.HasPrefix(key, "admin_") {
			targetJSON[key] = value
		}
		if strings.HasPrefix(key, "index.") {
			targetJSON[key] = value
		}
	}

	updatedData, err := json.MarshalIndent(targetJSON, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(targetPath, updatedData, 0644)
}
