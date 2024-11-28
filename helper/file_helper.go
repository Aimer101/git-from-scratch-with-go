package helper

import (
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func CompressIntoFile(file *os.File, body []byte) error {
	w := zlib.NewWriter(file)
	_, err := w.Write(body)
	if err != nil {
		return fmt.Errorf("error writing to zlip writer: %v", err)
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

// tree {treeSHa}
// parent {parentCommitSHA}
// author {author} <{email}> {currentUnixTime} {timezone}
// committer {author} <{email}> {currentUnixTime} {timezone}
// {commitMessage}
func CommitTree(treeSHA string, parentCommitSHA string, commitMessage string) (string, error) {
	author := "Foo bar"
	email := "foo@example.com"
	currentUnixTime := time.Now().Unix()
	timezone, _ := time.Now().Local().Zone()

	content := fmt.Sprintf("tree %s\nparent %s\nauthor %s <%s> %d %s\ncommitter %s <%s> %d %s\n\n%s\n",
		treeSHA,
		parentCommitSHA,
		author,
		email,
		currentUnixTime,
		timezone,
		author,
		email,
		currentUnixTime,
		timezone,
		commitMessage,
	)

	header := fmt.Sprintf("commit %d\000", len(content))
	fullContent := append([]byte(header), []byte(content)...)

	hash := sha1.Sum([]byte(fullContent))

	hashHex := hex.EncodeToString(hash[:])

	objectPath := filepath.Join(".git/objects", hashHex[:2])
	err := WriteIntoPath(objectPath, hashHex[2:], []byte(fullContent))

	if err != nil {
		return "", err
	}

	return hashHex, nil

}

// tree format is
//
//	tree <size>\0
//	<mode> <name>\0<20_byte_sha>
//	<mode> <name>\0<20_byte_sha>
func WriteTree(currentPath string) (string, error) {
	entries := []TreeEntry{}

	files, err := os.ReadDir(currentPath)

	if err != nil {
		return "", err
	}

	for _, file := range files {
		name := file.Name()

		if name == ".git" {
			continue
		}

		fullPath := filepath.Join(currentPath, name)

		if file.IsDir() {
			hash, err := WriteTree(fullPath)

			if err != nil {
				return "", err
			}

			entries = append(entries, TreeEntry{
				Mode: "40000",
				Name: name,
				SHA:  hash,
			})

		} else {
			hash, err := writeBlob(fullPath)

			if err != nil {
				return "", err
			}

			entries = append(entries, TreeEntry{
				Mode: "100644",
				Name: name,
				SHA:  hash,
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	// tree <size>\0
	// <mode> <name>\0<20_byte_sha>
	// <mode> <name>\0<20_byte_sha>
	var treeContent []byte

	for _, entry := range entries {
		hashBytes, err := hex.DecodeString(entry.SHA)

		if err != nil {
			return "", err
		}

		contentPrefix := fmt.Sprintf("%s %s\000", entry.Mode, entry.Name)
		treeContent = append(treeContent, []byte(contentPrefix)...)
		treeContent = append(treeContent, hashBytes...)
	}

	header := fmt.Sprintf("tree %d\000", len(treeContent))
	fullContent := append([]byte(header), treeContent...)

	hash := sha1.Sum(fullContent)
	hashHex := hex.EncodeToString(hash[:])

	objectPath := filepath.Join(".git/objects", hashHex[:2])
	err = WriteIntoPath(objectPath, hashHex[2:], fullContent)

	if err != nil {
		return "", err
	}

	return hashHex, nil
}

// blob <size>\0<content>,
func writeBlob(path string) (string, error) {
	file, err := os.ReadFile(path)

	if err != nil {
		return "", err
	}

	header := fmt.Sprintf("blob %d\000", len(file))
	fullContent := append([]byte(header), file...)

	hash := sha1.Sum(fullContent)
	hashHex := hex.EncodeToString(hash[:])

	objectPath := filepath.Join(".git/objects", hashHex[:2])

	err = WriteIntoPath(objectPath, hashHex[2:], fullContent)

	if err != nil {
		return "", err
	}

	return hashHex, nil

}

func InitialiseGitDirectory() {
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

}
