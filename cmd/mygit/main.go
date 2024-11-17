package main

import (
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
