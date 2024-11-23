package main

import (
	"crypto/sha1"
	"fmt"
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

	case "cat-file":
		sha := os.Args[3]

		// first 2 of sha is dir
		// 2: is the file

		fullPath := filepath.Join(".git/objects", sha[:2], sha[2:])

		file, err := os.Open(fullPath)

		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading file: %v\n", err)

		}

		defer file.Close()

		decompressedFile, err := helper.DecompressFile(file)

		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
		}

		// fmt.Println(string(r))

		res := strings.Split(string(decompressedFile), "\x00")

		// for index, line := range res {
		// 	fmt.Println(index, line)
		// }
		fmt.Print(res[1])

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

	case "ls-tree":
		tree_sha := os.Args[3]
		fullPath := filepath.Join(".git/objects", tree_sha[:2], tree_sha[2:])

		// Read file
		file, err := os.Open(fullPath) // <-- the file is compressed

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
			os.Exit(1)
		}
		defer file.Close()

		decompressedFile, err := helper.DecompressFile(file)

		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}

		res, err := helper.ParseTreeEntries(decompressedFile)

		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}

		for _, entry := range res {
			fmt.Println(entry.Name)
		}

	case "write-tree":
		currentDir, err := os.Getwd()

		if err != nil {
			fmt.Println("Error getting current directory:", err)
			os.Exit(1)

		}

		hash, err := helper.WriteTree(currentDir)

		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}

		fmt.Println(hash)

	case "commit-tree":
		treeSHA := os.Args[2]
		parentCommitSHA := os.Args[4]
		commitMessage := os.Args[6]

		hash, err := helper.CommitTree(treeSHA, parentCommitSHA, commitMessage)

		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}

		fmt.Println(hash)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}
