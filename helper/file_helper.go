package helper

import (
	"compress/zlib"
	"fmt"
	"os"
	"path/filepath"
)

func CompressIntoFile(file *os.File, body []byte) error {
	w := zlib.NewWriter(file)
	_, err := w.Write(body)
	if err != nil {
		return fmt.Errorf("error writing to gzip writer: %v", err)
	}

	w.Close()

	return nil
}

func WriteIntoPath(path string, filename string, body []byte) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("error creating object directory: %v", err)
	}

	fullPath := filepath.Join(path, filename)

	file, err := os.Create(fullPath)

	if err != nil {
		return fmt.Errorf("error creating object file: %v", err)
	}

	defer file.Close()

	err = CompressIntoFile(file, body)

	if err != nil {
		return err
	}

	return nil
}
