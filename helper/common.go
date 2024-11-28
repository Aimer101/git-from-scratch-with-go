package helper

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
)

func ArrayContains[T comparable](arr []T, item T) bool {
	for _, v := range arr {
		if v == item {
			return true
		}
	}
	return false
}

// get the object before compression
func ProcessObject(packfile []byte) (int, []byte, error) {
	b := bytes.NewReader(packfile)
	r, err := zlib.NewReader(b)
	if err != nil {
		return 0, nil, err
	}
	defer r.Close()
	object, err := io.ReadAll(r)

	if err != nil {
		return 0, nil, err
	}

	totalPackfileSize := len(packfile)
	remainingBytes := b.Len() // number of unread bytes

	processedOffset := int(totalPackfileSize) - remainingBytes

	return processedOffset, object, nil
}

func GetObjectSHA(blob []byte, objectType string) ([20]byte, []byte) {
	header := fmt.Sprintf("%s %d\000", objectType, len(blob))
	fullContent := append([]byte(header), blob...)

	hash := sha1.Sum(fullContent)

	return hash, fullContent
}

func SaveBlob(hash [20]byte, blob []byte) error {
	// create dir
	err := os.MkdirAll(fmt.Sprintf(".git/objects/%x/", hash[:1]), 0755)
	if err != nil {
		fmt.Println(err)
		return err
	}

	// compress file content
	compressed := bytes.Buffer{}
	writer := zlib.NewWriter(&compressed)
	writer.Write(blob)
	writer.Close()

	fullPath := fmt.Sprintf(".git/objects/%x/%x", hash[:1], hash[1:])

	err = os.WriteFile(fullPath, compressed.Bytes(), 0644)
	if err != nil {
		fmt.Println(err)
		return err
	}

	// fmt.Println("data is saved to ", fullPath)
	return nil
}
