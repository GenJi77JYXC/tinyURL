// Package base62 converts database auto-increment IDs into fixed-length,
// URL-safe short codes using the alphabet [0-9a-zA-Z].
//
// Every code is exactly CodeLength (6) characters: the raw sequence ID is
// shifted by MinID (62^5), so the encoded value always lives in [62^5, 62^6).
// That interval provides MaxID+1 = 62^6 - 62^5 ≈ 55.9 billion usable codes
// (the full 6-character space is 62^6 ≈ 56.8 billion).
package base62

import "errors"

const (
	alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	baseUint = uint64(62)

	// CodeLength is the guaranteed width of every generated code.
	CodeLength = 6

	// MinID = 62^5, the smallest value whose base-62 form has 6 digits.
	MinID = uint64(62 * 62 * 62 * 62 * 62) // 916,132,832

	// MaxID is the largest sequence ID that still maps to a 6-char code.
	MaxID = uint64(62*62*62*62*62*62) - 1 - MinID // 55,884,102,751
)

// ErrOverflow is returned when an ID exceeds the 6-character code space.
var ErrOverflow = errors.New("base62: id overflows the 6-character code space")

// ErrInvalidCode is returned when a code cannot be decoded.
var ErrInvalidCode = errors.New("base62: invalid short code")

// decodeTable maps ASCII bytes to alphabet indices; 255 means "not in alphabet".
var decodeTable [128]byte

func init() {
	for i := range decodeTable {
		decodeTable[i] = 255
	}
	for i := 0; i < len(alphabet); i++ {
		decodeTable[alphabet[i]] = byte(i)
	}
}

// Encode maps a sequence ID to a fixed-width 6-character base-62 code.
func Encode(id uint64) (string, error) {
	if id > MaxID {
		return "", ErrOverflow
	}

	n := id + MinID
	var buf [CodeLength]byte
	for i := CodeLength - 1; i >= 0; i-- {
		buf[i] = alphabet[n%baseUint]
		n /= baseUint
	}
	return string(buf[:]), nil
}

// Decode parses a 6-character code back into its original sequence ID.
func Decode(code string) (uint64, error) {
	if len(code) != CodeLength {
		return 0, ErrInvalidCode
	}

	var n uint64
	for i := 0; i < len(code); i++ {
		c := code[i]
		if c >= 128 || decodeTable[c] == 255 {
			return 0, ErrInvalidCode
		}
		n = n*baseUint + uint64(decodeTable[c])
	}

	if n < MinID {
		return 0, ErrInvalidCode
	}
	return n - MinID, nil
}
