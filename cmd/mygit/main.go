package main

import (
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/codecrafters-io/git-starter-go/helper"
)

func main() {

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: mygit <command> [<args>...]\n")
		os.Exit(1)
	}

	switch command := os.Args[1]; command {
	case "init":
		for _, dir := range []string{".git", ".git/objects", ".git/refs"} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating directory: %s\n", err)
			}
		}
		headFileContents := []byte("ref: refs/heads/main\n")
		if err := os.WriteFile(".git/HEAD", headFileContents, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %s\n", err)
		}
		fmt.Println("Initialized git directory")

	case "hash-object":
		writeFlag := false
		fileIndex := 2

		if os.Args[2] == "-w" {
			writeFlag = true
			fileIndex = 3
		}

		filePath := os.Args[fileIndex]
		content, err := os.ReadFile(filePath)

		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading file: %v\n", err)
			os.Exit(1)
		}

		blobHeader := fmt.Sprintf("blob %d\000", len(content)) // \x00 == \000 == null, it prints bloc {bytes length}
		byteBlobHeader := []byte(blobHeader)
		blobObject := append(byteBlobHeader, content...) // appending bytes of the content into the blob header

		// Compute SHA1 hash
		hash := sha1.Sum(blobObject)
		hashStr := fmt.Sprintf("%x", hash)

		// Print hash
		fmt.Println(hashStr)

		if writeFlag {
			fullPath := filepath.Join(".git/objects", hashStr[:2])

			// Create zlib writer
			err := helper.WriteIntoPath(fullPath, hashStr[2:], blobObject)

			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}

		}

	case "cat-file":
		sha := os.Args[3]

		// first 2 of sha is dir
		// 2: is the file

		fullPath := filepath.Join(".git/objects", sha[:2], sha[2:])

		file, err := os.Open(fullPath)

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
		}

		decompressor, err := zlib.NewReader(file) // create decompressor

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
		}

		r, err := io.ReadAll(decompressor) // read decompressed data

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
		}

		decompressor.Close()
		// fmt.Println(string(r))

		res := strings.Split(string(r), "\x00")

		// for index, line := range res {
		// 	fmt.Println(index, line)
		// }
		fmt.Print(res[1])

	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}
