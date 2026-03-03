package textfile

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"unicode/utf16"
	"unicode/utf8"
)

const sniffBytes = 8192

var ErrBinary = errors.New("binary or non-text file")
var ErrEmpty = errors.New("empty file")

func Read(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	head := make([]byte, sniffBytes)
	n, err := file.Read(head)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	head = head[:n]

	if len(head) == 0 {
		return "", ErrEmpty
	}

	data, err := io.ReadAll(io.MultiReader(bytes.NewReader(head), file))
	if err != nil {
		return "", err
	}

	text, ok := decodeText(data)
	if !ok {
		return "", ErrBinary
	}
	if text == "" {
		return "", ErrEmpty
	}
	return text, nil
}

func decodeText(data []byte) (string, bool) {
	if len(data) >= 3 && bytes.Equal(data[:3], []byte{0xEF, 0xBB, 0xBF}) {
		trimmed := data[3:]
		if utf8.Valid(trimmed) {
			return string(trimmed), true
		}
		return "", false
	}

	if len(data) >= 2 && bytes.Equal(data[:2], []byte{0xFF, 0xFE}) {
		return decodeUTF16(data[2:], true)
	}
	if len(data) >= 2 && bytes.Equal(data[:2], []byte{0xFE, 0xFF}) {
		return decodeUTF16(data[2:], false)
	}

	if bytes.ContainsRune(data, '\x00') {
		return "", false
	}

	if utf8.Valid(data) {
		return string(data), true
	}

	return "", false
}

func decodeUTF16(data []byte, littleEndian bool) (string, bool) {
	if len(data)%2 != 0 {
		return "", false
	}
	words := make([]uint16, 0, len(data)/2)
	for i := 0; i < len(data); i += 2 {
		if data[i] == 0 && data[i+1] == 0 {
			return "", false
		}
		var w uint16
		if littleEndian {
			w = uint16(data[i]) | uint16(data[i+1])<<8
		} else {
			w = uint16(data[i])<<8 | uint16(data[i+1])
		}
		words = append(words, w)
	}

	runes := utf16.Decode(words)
	if !utf8.ValidString(string(runes)) {
		return "", false
	}
	return string(runes), true
}

func Reason(err error) string {
	switch {
	case errors.Is(err, ErrBinary):
		return "binary"
	case errors.Is(err, ErrEmpty):
		return "empty"
	case err != nil:
		return fmt.Sprintf("error: %v", err)
	default:
		return ""
	}
}
