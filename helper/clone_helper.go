package helper

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// because flush character stuck together with the response,
func ParsePacketLines(data []byte) []string {
	var lines []string
	pointer := 0

	for pointer < len(data) {
		// Need at least 4 bytes for the length prefix
		if pointer+4 > len(data) {
			break
		}

		lengthHex := string(data[pointer : pointer+4])
		length, err := strconv.ParseInt(lengthHex, 16, 32)

		if err != nil {
			break
		}
		// Length of 0 ("0000") indicates a flush packet
		if length == 0 {
			pointer += 4
			continue
		}

		// eg, 001e# service=git-upload-pack\n, meaning length is 001e, (30 in decimal) including itself
		contentStart := pointer + 4
		contentEnd := pointer + int(length)

		line := string(data[contentStart:contentEnd])

		lines = append(lines, line)

		pointer = contentEnd
	}

	return lines
}

func RetrieveMainSHA(lines []string) (string, error) {

	for _, line := range lines {
		if strings.HasPrefix(line, "# service=git-upload-pack") {
			continue
		}

		// for logging purpose, can ignore
		if strings.Contains(line, "symref=HEAD:") {
			parts := strings.Split(line, "symref=HEAD:")
			defaultBranch := strings.Split(parts[1], " ")[0]

			fmt.Println("Default branch is ", defaultBranch)
		}

		if strings.Contains(line, "HEAD") {
			sha := line[:40]
			return sha, nil

		}

	}

	return "", fmt.Errorf("no main ref found")
}

// 7 	-> 0000 0111
// 15 	-> 0000 1111
// 0x80 -> 1000 0000
// 0x7f -> 0111 1111
//
//	STTT LLLL
//
// 7| 6| 5| 4| 3| 2| 1|0
// S (1 bit) = Size extension bit (indicates if more length bytes follow)
// T (3 bits) = Type of the object
// L (4 bits) = Length of the object (only in the first byte, more bits may follow)
func ReadObjectHeader(data []byte) (string, int64, int, error) {
	objectType := (data[0] >> 4) & 7
	size := int64(data[0] & 15)

	shift := uint(4)
	headerOffset := 1

	// if MSB is not 0, we need to find more bit for length of object
	for data[headerOffset-1]&0x80 != 0 {
		if headerOffset >= len(data) {
			return "", 0, 0, fmt.Errorf("premature end of header data")
		}

		// Extract the lower 7 bits (without the continuation bit)
		currentByte := data[headerOffset] & 0x7f

		// Shift the extracted bits by the current shift value
		shiftedBits := int64(currentByte) << shift

		// Combine the shifted bits with the existing size
		size += shiftedBits

		shift += 7 // add 7 bits to the shift for next iteration
		headerOffset++
	}

	// Convert type number to string
	typeStr := ""
	switch objectType {
	case 1:
		typeStr = "commit"
	case 2:
		typeStr = "tree"
	case 3:
		typeStr = "blob"
	case 4:
		typeStr = "tag"
	case 6:
		typeStr = "ofs-delta"
	case 7:
		typeStr = "ref-delta"
	default:
		return "", 0, 0, fmt.Errorf("unknown object type: %d", objectType)
	}

	return typeStr, size, headerOffset, nil
}

func WriteObject(hash [20]byte, blob []byte) error {

	err := os.MkdirAll(fmt.Sprintf(".git/objects/%x/", hash[:1]), 0755)
	if err != nil {
		fmt.Println(err)
		return err
	}

	compressed := bytes.Buffer{}
	writer := zlib.NewWriter(&compressed)
	writer.Write(blob)
	writer.Close()

	err = os.WriteFile(fmt.Sprintf(".git/objects/%x/%x", hash[:1], hash[1:]), compressed.Bytes(), 0644)
	if err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}

//	SLLL LLLL
//
// S (1 bit) = Size extension bit (indicates if more length bytes follow)
// L (7 bits) = Length of the object (only in the first byte, more bits may follow)
func readSize(packfile []byte) (uint64, int, error) {
	offset := 0
	data := packfile[offset]
	size := uint64(data & 0x7F)

	shift := uint(7)
	offset += 1

	for data&0x80 != 0 {
		if offset >= len(packfile) {
			return 0, 0, fmt.Errorf("premature end of header data")
		}

		data = packfile[offset]
		size += uint64(data&0x7F) << shift

		offset++
		shift += 7
	}
	return size, offset, nil
}

func OpenObject(objectName string) ([]byte, string, error) {
	file, err := os.Open(fmt.Sprintf(".git/objects/%s/%s", objectName[:2], objectName[2:]))

	if err != nil {
		return nil, "", err
	}

	defer file.Close()

	reader, err := zlib.NewReader(file)

	if err != nil {
		return nil, "", err
	}

	defer reader.Close()

	data, err := io.ReadAll(reader)

	if err != nil {
		return nil, "", err
	}

	// Find header end
	headerEnd := bytes.IndexByte(data, '\000')

	if headerEnd == -1 {
		return nil, "", errors.New("invalid object header")
	}

	var (
		objectType string
		size       int
	)
	// <object type> <length>
	fmt.Sscanf(string(data[:headerEnd]), "%s %d", &objectType, &size)

	return data[headerEnd+1:], objectType, nil
}

// instruction: <base object size> <expected object size>
func BuildDeltaObject(baseObject []byte, buildInstruction []byte) ([]byte, error) {

	offset := 0

	// Read base object
	baseSize, processedOffset, err := readSize(buildInstruction[offset:])
	if err != nil {
		return nil, err
	}
	offset += int(processedOffset)

	if len(baseObject) != int(baseSize) {
		fmt.Println("delta header size does not match base object size")
		return nil, errors.New("delta header size does not match base object size")

	}

	// read expected size after build
	expectedSize, processedOffset, err := readSize(buildInstruction[offset:])
	if err != nil {
		return nil, err
	}

	offset += int(processedOffset)

	buffer := bytes.Buffer{}

	for offset < len(buildInstruction) {
		// In the context of Git's delta compression,
		// an "opcode" is a single byte that instructs how to reconstruct the target object from the base object.
		// There are two primary types of opcodes in this delta encoding:
		// 1. "Copy Instruction Opcode (0x80)
		// 2. Insert Instruction Opcode (0)
		opcode := buildInstruction[offset]
		offset++

		// mask = 1000 0000
		if opcode&0x80 != 0 {
			// Copy instruction: copy from base object arg
			// Parse additional bytes to get offset and size

			// 	+----------+---------+---------+---------+---------+-------+-------+-------+
			// 	| 1xxxxxxx | offset1 | offset2 | offset3 | offset4 | size1 | size2 | size3 |
			// 	+----------+---------+---------+---------+---------+-------+-------+-------+

			//   This is the instruction format to copy a byte range from the source
			//   object. It encodes the offset to copy from and the number of bytes to
			//   copy. Offset and size are in little-endian order.

			//   All offset and size bytes are optional. This is to reduce the
			//   instruction size when encoding small offsets or sizes. The first seven
			//   bits in the first octet determines which of the next seven octets is
			//   present. If bit zero is set, offset1 is present. If bit one is set
			//   offset2 is present and so on.

			//   Note that a more compact instruction does not change offset and size
			//   encoding. For example, if only offset2 is omitted like below, offset3
			//   still contains bits 16-23. It does not become offset2 and contains
			//   bits 8-15 even if it's right next to offset1.
			var props uint64
			for bit := 0; bit < 7; bit++ {
				if opcode&(1<<bit) != 0 {
					currentInstructionByte := buildInstruction[offset]
					currentInstruction := uint64(currentInstructionByte)

					if currentInstructionByte != 0 {
						currentInstruction = currentInstruction << (bit * 8)
						props += currentInstruction
					}
					offset++
				}
			}

			// mask = 11111111 11111111 11111111 11111111
			startIndexToCopy := props & 0xFFFFFFFF // extract lower 32 bits

			// mask = 11111111 11111111 11111111
			sizeOfObjectToCopy := (props >> 32) & 0xFFFFFF // extract upper 32 bits

			buffer.Write(baseObject[startIndexToCopy : startIndexToCopy+sizeOfObjectToCopy])
		} else {
			// insert instruction : insert from the instruction arg
			// 	+----------+============+
			// 	| 0xxxxxxx |    data    |
			// 	+----------+============+

			//   This is the instruction to construct target object without the base
			//   object. The following data is appended to the target object. The first
			//   seven bits of the first octet determines the size of data in
			//   bytes. The size must be non-zero.

			// size is last 7 bits
			size := int(opcode & 0x7F)
			buffer.Write(buildInstruction[offset : offset+size])
			offset += size
		}
	}

	undeltifiedObject := buffer.Bytes()

	if int(expectedSize) != len(undeltifiedObject) {
		fmt.Println("expected size is not equal to undeltified object size")
		return nil, errors.New("expected size is not equal to undeltified object size")
	}

	return undeltifiedObject, nil
}
