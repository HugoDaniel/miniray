// Package sourcemap provides source map generation for the WGSL minifier.
//
// It implements the Source Map v3 format as specified at:
// https://sourcemaps.info/spec.html
package sourcemap

import (
	"errors"
	"strings"
)

// Base64 alphabet used for VLQ encoding in source maps
const base64Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

// base64Values is a lookup table for decoding base64 characters
var base64Values [128]int

func init() {
	// Initialize lookup table with -1 for invalid characters
	for i := range base64Values {
		base64Values[i] = -1
	}
	// Set valid character values
	for i, c := range base64Alphabet {
		base64Values[c] = i
	}
}

// VLQ constants
const (
	vlqBaseShift       = 5
	vlqBase            = 1 << vlqBaseShift        // 32
	vlqBaseMask        = vlqBase - 1              // 31 (0x1F)
	vlqContinuationBit = vlqBase                  // 32 (0x20)
	vlqSignBit         = 1
)

// EncodeVLQ encodes a signed integer as a VLQ base64 string.
// The encoding follows the source map v3 specification.
func EncodeVLQ(value int) string {
	var buf strings.Builder

	// Convert to VLQ signed representation:
	// - Positive numbers: value << 1
	// - Negative numbers: ((-value) << 1) | 1
	var vlq uint32
	if value < 0 {
		vlq = uint32((-value) << 1) | vlqSignBit
	} else {
		vlq = uint32(value << 1)
	}

	// Encode as base64 VLQ
	for {
		digit := vlq & vlqBaseMask
		vlq >>= vlqBaseShift

		if vlq > 0 {
			// More digits to come, set continuation bit
			digit |= vlqContinuationBit
		}

		buf.WriteByte(base64Alphabet[digit])

		if vlq == 0 {
			break
		}
	}

	return buf.String()
}

// DecodeVLQ decodes a VLQ base64 string and returns the value and bytes consumed.
// Returns (0, 0) if the input is empty or invalid.
func DecodeVLQ(input string) (int, int) {
	if len(input) == 0 {
		return 0, 0
	}

	var vlq uint32
	var shift uint32
	consumed := 0

	for i := 0; i < len(input); i++ {
		c := input[i]
		if c >= 128 {
			return 0, 0 // Invalid character
		}

		digit := base64Values[c]
		if digit < 0 {
			return 0, 0 // Invalid character
		}

		// Check for continuation bit
		continuation := (digit & vlqContinuationBit) != 0
		digit &= vlqBaseMask

		vlq |= uint32(digit) << shift
		shift += vlqBaseShift
		consumed++

		if !continuation {
			// Convert from VLQ signed representation
			negative := (vlq & vlqSignBit) != 0
			vlq >>= 1

			if negative {
				return -int(vlq), consumed
			}
			return int(vlq), consumed
		}
	}

	// Truncated input (continuation bit set but no more data)
	return 0, 0
}

// EncodeVLQSequence encodes multiple values as a VLQ sequence.
func EncodeVLQSequence(values []int) string {
	var buf strings.Builder
	for _, v := range values {
		buf.WriteString(EncodeVLQ(v))
	}
	return buf.String()
}

// DecodeVLQSequence decodes a VLQ sequence expecting n values.
// Returns an error if not enough values can be decoded.
func DecodeVLQSequence(input string, n int) ([]int, error) {
	values := make([]int, 0, n)
	pos := 0

	for i := 0; i < n; i++ {
		if pos >= len(input) {
			return nil, errors.New("unexpected end of VLQ sequence")
		}

		value, consumed := DecodeVLQ(input[pos:])
		if consumed == 0 {
			return nil, errors.New("invalid VLQ encoding")
		}

		values = append(values, value)
		pos += consumed
	}

	return values, nil
}
