package helper

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

type TreeEntry struct {
	Mode string
	Name string
	SHA  string
}

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
