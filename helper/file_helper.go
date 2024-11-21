package helper

import (
	"compress/zlib"
	"fmt"
	"io"
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

func DecompressFile(file *os.File) ([]byte, error) {
	decompressor, err := zlib.NewReader(file) // create decompressor
	if err != nil {
		return nil, fmt.Errorf("error creating zlib reader: %v", err)
	}

	defer decompressor.Close()

	r, err := io.ReadAll(decompressor) // read decompressed data

	if err != nil {
		return nil, fmt.Errorf("error reading from zlib reader: %v", err)
	}

	return r, nil
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
