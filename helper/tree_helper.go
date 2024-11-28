package helper

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type TreeEntry struct {
	Mode string
	Name string
	SHA  string
}

// parse tree entries
// tree <size>\0
// <mode> <name>\0<20_byte_sha>
// <mode> <name>\0<20_byte_sha>
func ParseTreeEntries(data []byte) ([]TreeEntry, error) {

	headerEnd := bytes.IndexByte(data, '\000')

	var result []TreeEntry

	if headerEnd == -1 {
		return nil, fmt.Errorf("error: could not find null byte after header")
	}

	offset := headerEnd + 1

	for offset < len(data) {
		nullByteIndex := bytes.IndexByte(data[offset:], '\000')
		if nullByteIndex == -1 {
			return nil, fmt.Errorf("error: could not find null byte after entry")
		}

		// Extract the mode + name
		modeName := data[offset : offset+nullByteIndex]
		offset += nullByteIndex + 1

		shaBytes := data[offset : offset+20]
		offset += 20

		splittedModeName := strings.Split(string(modeName), " ")

		mode := string(splittedModeName[0])
		name := string(splittedModeName[1])

		result = append(result, TreeEntry{
			Mode: mode,
			Name: name,
			SHA:  fmt.Sprintf("%x", shaBytes),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil

}

func CheckoutTree(treeHash, dir string) error {
	os.MkdirAll(dir, 0755)

	fullPath := filepath.Join(".git/objects", treeHash[:2], treeHash[2:])

	// Read file
	file, err := os.Open(fullPath) // <-- the file is compressed

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
		os.Exit(1)
	}

	defer file.Close()

	decompressedFile, err := DecompressFile(file)

	if err != nil {
		return err
	}

	// tree <size>\0
	// <mode> <name>\0<20_byte_sha>
	// <mode> <name>\0<20_byte_sha>
	res, err := ParseTreeEntries(decompressedFile)

	if err != nil {
		return err
	}

	for _, entry := range res {
		hashStr := entry.SHA
		mode := entry.Mode
		name := entry.Name
		path := fmt.Sprintf("%s/%s", dir, name)

		// recursively checkout tree since this is a dir
		if entry.Mode == "40000" {
			err = CheckoutTree(hashStr, path)
			if err != nil {
				return err
			}
		} else if mode == "100644" || mode == "100755" /* file */ {
			blob, objectType, err := OpenObject(hashStr)
			if err != nil {
				fmt.Println(err)
				return err
			}
			if objectType != "blob" {
				fmt.Println(err)
				return errors.New("object not a blob")
			}
			os.WriteFile(path, blob, 0644)
		}

	}

	return nil
}
