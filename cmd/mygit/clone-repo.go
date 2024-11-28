package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/codecrafters-io/git-starter-go/helper"
)

type DeltifiedObject struct {
	baseObjectSHA string
	instruction   []byte
}

func CloneRepo(repoUrl string, path string) error {

	// 1. make dir
	if err := os.MkdirAll(path, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directory: %s\n", err)
	}

	// 2. cd into the created folder
	err := os.Chdir(path)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error changing directory: %s\n", err)
	}

	// 3. initialise git
	helper.InitialiseGitDirectory()

	// 4. ref discovery
	fmt.Println("initialise ref discovery")
	hash, error := refDiscovery(repoUrl)

	if error != nil {
		return error
	}

	// 4. request pack file
	fmt.Println("initialise request pack file")
	packFile, error := requestPackFile(repoUrl, hash)

	if error != nil {
		return error
	}

	// 5. process pack file, build .git objects
	// the first object should be same as the hash from ref discovery
	fmt.Println("process pack file")
	err = processPacketFile(packFile)

	if err != nil {
		fmt.Println("error while processing pack file ", err)
	}

	// 6. create files and dirs from objects with the hash from ref discovery,
	//    note that the hash from ref discovery step == the hash of the first object in the pack file
	err = checkoutCommit(hash)

	return err
}

// this first request is to get the hash which
// is needed for later api request to get the packFile
// 2. Get repository info using Smart HTTP protocol -> C: GET $GIT_URL/info/refs?service=git-upload-pack HTTP/1.0
// response is like below
// S: 200 OK
// S: Content-Type: application/x-git-upload-pack-advertisement
// S: Cache-Control: no-cache
// S:
// S: 001e# service=git-upload-pack\n
// S: 0000
// S: 004895dcfa3633004da0049d3d0fa03f80589cbcaf31 refs/heads/maint\0multi_ack\n
// S: 003fd049f6c27a2244e12041955e262a404c7faba355 refs/heads/master\n
// S: 003c2cb58b79488a98d2721cea644875a8dd0026b115 refs/tags/v1.0\n
// S: 003fa3c2e2402b99163d1d59756e5f207ae21cccba4c refs/tags/v1.0^{}\n
// S: 0000
// remember that the first 4 bytes are the length of the response
// eg, 001e# service=git-upload-pack\n, meaning length is 001e, (30 in decimal) including itself
func refDiscovery(repoUrl string) (string, error) {
	infoUrl := repoUrl + "/info/refs?service=git-upload-pack"
	res, err := http.Get(infoUrl)

	if err != nil {
		return "", fmt.Errorf("error getting repository info: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status %d", res.StatusCode)
	}
	// response:
	// 001e# service=git-upload-pack
	// 0000015547b37f1a82bfe85f6d8df52b6258b75e4343b7fd HEADmulti_ack thin-pack side-band side-band-64k ofs-delta shallow deepen-since deepen-not deepen-relative no-progress include-tag multi_ack_detailed allow-tip-sha1-in-want allow-reachable-sha1-in-want no-done symref=HEAD:refs/heads/master filter object-format=sha1 agent=git/github-50ee4bdaf298
	// we want the hash
	body, err := io.ReadAll(res.Body)

	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	// helper that separate the length and the response into string array
	// arr[0] -> # service=git-upload-pack
	// arr[1] -> 47b37f1a82bfe85f6d8df52b6258b75e4343b7fd HEADmulti_ack thin-pack side-band side-band-64k ofs-delta shallow deepen-since deepen-not deepen-relative no-progress include-tag multi_ack_detailed allow-tip-sha1-in-want allow-reachable-sha1-in-want no-done symref=HEAD:refs/heads/master filter object-format=sha1 agent=git/github-50ee4bdaf298
	// arr[2] -> 47b37f1a82bfe85f6d8df52b6258b75e4343b7fd refs/heads/master
	lines := helper.ParsePacketLines(body)

	// retrieve the SHA
	hash, error := helper.RetrieveMainSHA(lines)

	if error != nil {
		return "", error
	}

	return hash, nil
}

// the request protocol is based on https://git-scm.com/docs/http-protocol
// 	Smart Service git-receive-pack
// This service reads from the repository pointed to by $GIT_URL.

// Clients MUST first perform ref discovery with $GIT_URL/info/refs?service=git-receive-pack.

// C: POST $GIT_URL/git-receive-pack HTTP/1.0
// C: Content-Type: application/x-git-receive-pack-request
// C:
// C: ....0a53e9ddeaddad63ad106860237bbf53411d11a7 441b40d833fdfa93eb2908e52742248faf0ee993 refs/heads/maint\0 report-status
// C: 0000
// C: PACK....
// The response will start with a NAK or ACK, followed by the packfile
// Find where the packfile starts (it starts with "PACK")
// packStart := bytes.Index(packData, []byte("PACK"))
func requestPackFile(repoUrl string, hash string) ([]byte, error) {
	uploadPackUrl := repoUrl + "/git-upload-pack"

	// The length prefix "0032" represents 50 bytes (32 in hex): 4 bytes for length + "want " (5 bytes) + hash (40 bytes) + "\n" (1 byte)
	wantLine := fmt.Sprintf("0032want %s\n", hash)

	// Add a flush packet after the want line
	flushPkt := "0000"

	// Add the done line - "0009" represents 9 bytes: 4 bytes for length + "done\n" (5 bytes)
	doneLine := "0009done\n"

	// Combine all parts of the request
	requestBody := wantLine + flushPkt + doneLine

	req, err := http.NewRequest("POST", uploadPackUrl, strings.NewReader(requestBody))

	if err != nil {
		return []byte{}, fmt.Errorf("error creating upload-pack request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-git-upload-pack-request")

	client := &http.Client{}
	res, err := client.Do(req)

	if err != nil {
		return []byte{}, fmt.Errorf("error during upload-pack request: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return []byte{}, fmt.Errorf("upload-pack request failed with status: %d", res.StatusCode)
	}

	// s: 0008NAK
	// s: PACK<header><objects>
	packData, err := io.ReadAll(res.Body)

	if err != nil {
		return []byte{}, fmt.Errorf("error reading upload-pack response: %w", err)
	}

	// remove 0008NAK\n header and get the rest
	// so we need the offset after the header
	packetLength := packData[:4] // 0008
	offset, err := strconv.ParseInt(string(packetLength), 16, 32)
	if err != nil {
		return nil, err
	}

	return packData[offset:], nil
}

// https://github.com/git/git/blob/795ea8776befc95ea2becd8020c7a284677b4161/Documentation/gitformat-pack.txt
func processPacketFile(packFile []byte) error {
	// 4-byte signature:
	// The signature is: {'P', 'A', 'C', 'K'}
	if !bytes.HasPrefix(packFile, []byte("PACK")) {
		return fmt.Errorf("invalid packfile: missing PACK signature")
	}

	fmt.Println("PACK signature found")

	// followed by 4-byte version number (network byte order):
	// Git currently accepts version number 2 or 3 but
	// generates version 2 only.
	version := binary.BigEndian.Uint32(packFile[4:8])

	if version != 2 {
		return fmt.Errorf("unsupported packfile version: %d", version)
	}

	// followerd by 4-byte which represents number of objects contained in the pack (network byte order)
	// Observation: we cannot have more than 4G versions ;-) and
	//  more than 4G objects in a pack.
	numObjects := binary.BigEndian.Uint32(packFile[8:12])
	fmt.Printf("Packfile contains %d objects\n", numObjects)

	offset := 12
	var processedObject uint32
	deltaObjects := []DeltifiedObject{}

	for offset < len(packFile) {
		objectType, size, headerOffset, err := helper.ReadObjectHeader(packFile[offset:])

		if err != nil {
			return fmt.Errorf("error reading object header: %w", err)
		}

		offset += headerOffset

		if helper.ArrayContains([]string{"commit", "tree", "blob", "tag"}, objectType) {
			processedObjectOffset, blob, err := helper.ProcessObject(packFile[offset:])

			if err != nil {
				// todo
				// for some reason
				// the last object cannot be processed by zlib
				break
			}

			offset += int(processedObjectOffset)

			objectSha, objectContent := helper.GetObjectSHA(blob, objectType)

			err = helper.SaveBlob(objectSha, objectContent)

			if err != nil {
				return err
			}
		} else if objectType == "ofs-delta" {
			fmt.Println("type of object is ofs-delta")
			// n-byte offset (see below) interpreted as a negative
			// offset from the type-byte of the header of the
			// ofs-delta entry (the size above is the size of
			// the delta data that follows).
			// tldr: it tells us where the base object is
			return fmt.Errorf("todo: ofs-delta")
		} else if objectType == "ref-delta" {
			// base object name (the size above is the
			// 	size of the delta data that follows).
			// 	  delta data, deflated.
			// structure <header><base object sha><intruction to build>
			// eli5 verson:
			// For a REF_DELTA, Git gives:
			// The SHA-1 hash of the base object (the one you need to start from).
			// A delta recipe (compressed!) for transforming that base object into the current object.
			// So instead of saying "Go backward 180 steps," it says something like this:
			// "Hey, go find the object with the name abc123... in the Git database. Once you find it, apply this recipe (delta) to it."
			hash := packFile[offset : offset+20]
			offset += 20

			processedOffset, intruction, err := helper.ProcessObject(packFile[offset:])

			if err != nil {
				fmt.Println("here here error is ", err)
				return err
			}

			if int(size) != len(intruction) {
				return fmt.Errorf("object length doesnt match with header")
			}

			offset += processedOffset

			deltaObjects = append(deltaObjects, DeltifiedObject{instruction: intruction, baseObjectSHA: hex.EncodeToString(hash)})
		} else {
			fmt.Println("error unknown object type ", objectType)
			return fmt.Errorf("unknown object type: %s", objectType)
		}

		processedObject++
	}

	if numObjects != processedObject {
		return fmt.Errorf("object count in the headers doesnt match actual object count")
	}

	fmt.Println("delta object length are ", len(deltaObjects))

	// assume that delta object are in order
	for _, delta := range deltaObjects {
		baseObject, objectType, err := helper.OpenObject(delta.baseObjectSHA)

		if err != nil {
			return err
		}

		// err = helper.WriteDeltaObject(baseObject, delta.instruction, objectType)
		// if err != nil {
		// 	return err
		// }
		undeltifiedObject, err := helper.BuildDeltaObject(baseObject, delta.instruction)

		if err != nil {
			return err
		}

		objectSha, objectContent := helper.GetObjectSHA(undeltifiedObject, objectType)

		err = helper.SaveBlob(objectSha, objectContent)

		if err != nil {
			return err
		}
	}

	// some object is based on other delta object
	// for len(deltaObjects) > 0 {
	// 	unaddedDeltaObjects := []DeltaObject{}
	// 	added := false
	// 	for _, delta := range deltaObjects {
	// 		if helper.ObjectExists(delta.baseObjectSHA) {
	// 			added = true
	// 			baseObject, objectType, err := helper.OpenObject(delta.baseObjectSHA)
	// 			if err != nil {
	// 				return err
	// 			}
	// 			err = helper.WriteDeltaObject(baseObject, delta.instruction, objectType)
	// 			if err != nil {
	// 				return err
	// 			}
	// 		} else {
	// 			unaddedDeltaObjects = append(unaddedDeltaObjects, delta)
	// 		}
	// 	}
	// 	if !added {
	// 		return errors.New("bad delta objects")
	// 	}
	// 	deltaObjects = unaddedDeltaObjects
	// }

	return nil

}

func checkoutCommit(commitHash string) error {
	commit, objectType, err := helper.OpenObject(commitHash)
	if err != nil {
		return err
	}
	if objectType != "commit" {
		return errors.New("object not a commit")
	}

	// tree 0f99f9c5b83b010cfbd67870502df7b293ec0e37
	// parent 40c614ba65a7faf2c97a52a2fa74568dabc49ebb
	// author Paul Kuruvilla <rohitpaulk@gmail.com> 1587572148 +0530
	// committer Paul Kuruvilla <rohitpaulk@gmail.com> 1587572148 +0530

	// we need to extract the tree 40 characters hash
	spaceIndex := strings.Index(string(commit), " ")

	if spaceIndex == -1 {
		return errors.New("no space found")
	}

	startIndex := spaceIndex + 1

	treeHash := commit[startIndex : startIndex+40]

	err = helper.CheckoutTree(string(treeHash), ".")

	return err
}
