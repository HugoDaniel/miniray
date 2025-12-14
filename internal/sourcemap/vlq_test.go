package sourcemap

import (
	"fmt"
	"testing"
)

// ============================================================================
// VLQ Encoding Tests
// ============================================================================

func TestVLQEncodeZero(t *testing.T) {
	result := EncodeVLQ(0)
	if result != "A" {
		t.Errorf("EncodeVLQ(0) = %q, want %q", result, "A")
	}
}

func TestVLQEncodePositive(t *testing.T) {
	tests := []struct {
		value    int
		expected string
	}{
		{1, "C"},
		{2, "E"},
		{3, "G"},
		{15, "e"},
		{16, "gB"},
		{31, "+B"},
		{32, "gC"},
		{100, "oG"},   // 100 << 1 = 200 = 6*32 + 8 → 'o' (8|32=40) + 'G' (6)
		{1000, "w+B"}, // 1000 << 1 = 2000
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("value_%d", tt.value), func(t *testing.T) {
			result := EncodeVLQ(tt.value)
			if result != tt.expected {
				t.Errorf("EncodeVLQ(%d) = %q, want %q", tt.value, result, tt.expected)
			}
		})
	}
}

func TestVLQEncodeNegative(t *testing.T) {
	tests := []struct {
		value    int
		expected string
	}{
		{-1, "D"},
		{-2, "F"},
		{-15, "f"},
		{-16, "hB"},
		{-31, "/B"},
		{-32, "hC"},
		{-100, "pG"},   // (-100 << 1) | 1 = 201
		{-1000, "x+B"}, // (-1000 << 1) | 1 = 2001
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("value_%d", tt.value), func(t *testing.T) {
			result := EncodeVLQ(tt.value)
			if result != tt.expected {
				t.Errorf("EncodeVLQ(%d) = %q, want %q", tt.value, result, tt.expected)
			}
		})
	}
}

func TestVLQEncodeLarge(t *testing.T) {
	// Test large values that require multiple continuation bytes
	tests := []struct {
		value int
	}{
		{10000},
		{-10000},
		{100000},
		{-100000},
		{1000000},
		{-1000000},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("value_%d", tt.value), func(t *testing.T) {
			result := EncodeVLQ(tt.value)
			// Just verify it produces non-empty output and roundtrips
			if result == "" {
				t.Errorf("EncodeVLQ(%d) produced empty string", tt.value)
			}
			decoded, consumed := DecodeVLQ(result)
			if decoded != tt.value {
				t.Errorf("Roundtrip failed: %d -> %q -> %d", tt.value, result, decoded)
			}
			if consumed != len(result) {
				t.Errorf("DecodeVLQ consumed %d bytes, expected %d", consumed, len(result))
			}
		})
	}
}

func TestVLQDecodeBasic(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		consumed int
	}{
		{"A", 0, 1},
		{"C", 1, 1},
		{"D", -1, 1},
		{"e", 15, 1},
		{"f", -15, 1},
		{"gB", 16, 2},
		{"hB", -16, 2},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("input_%s", tt.input), func(t *testing.T) {
			value, consumed := DecodeVLQ(tt.input)
			if value != tt.expected || consumed != tt.consumed {
				t.Errorf("DecodeVLQ(%q) = (%d, %d), want (%d, %d)",
					tt.input, value, consumed, tt.expected, tt.consumed)
			}
		})
	}
}

func TestVLQRoundtrip(t *testing.T) {
	// Test encode-decode roundtrip for various values
	values := []int{
		0, 1, -1, 2, -2, 15, -15, 16, -16, 31, -31, 32, -32,
		100, -100, 1000, -1000, 10000, -10000,
		65536, -65536, 1000000, -1000000,
	}

	for _, v := range values {
		t.Run(fmt.Sprintf("value_%d", v), func(t *testing.T) {
			encoded := EncodeVLQ(v)
			decoded, consumed := DecodeVLQ(encoded)
			if decoded != v {
				t.Errorf("Roundtrip failed: %d -> %q -> %d", v, encoded, decoded)
			}
			if consumed != len(encoded) {
				t.Errorf("Did not consume all bytes: consumed %d of %d", consumed, len(encoded))
			}
		})
	}
}

func TestVLQSequence(t *testing.T) {
	tests := []struct {
		name     string
		values   []int
		expected string
	}{
		{"all_zeros", []int{0, 0, 0, 0}, "AAAA"},
		{"single_value", []int{5}, "K"},
		{"mixed", []int{0, 1, 2, 3}, "ACEG"},
		{"with_negatives", []int{0, -1, 0, 1}, "ADAC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EncodeVLQSequence(tt.values)
			if result != tt.expected {
				t.Errorf("EncodeVLQSequence(%v) = %q, want %q", tt.values, result, tt.expected)
			}
		})
	}
}

func TestVLQEmptyDecode(t *testing.T) {
	_, consumed := DecodeVLQ("")
	if consumed != 0 {
		t.Errorf("DecodeVLQ(\"\") consumed %d bytes, expected 0", consumed)
	}
}

func TestVLQInvalidContinuation(t *testing.T) {
	// VLQ with continuation bit set but truncated
	// Character with bit 5 set (continuation) but no following byte
	// In base64 VLQ, characters >= 32 (index) have continuation bit
	// 'g' is index 32, which has continuation bit set
	// If we only provide 'g', it should fail gracefully
	_, consumed := DecodeVLQ("g")
	// Should return 0 consumed on truncated input
	if consumed != 0 {
		t.Errorf("Truncated VLQ should return 0 consumed, got %d", consumed)
	}
}

func TestVLQBase64Alphabet(t *testing.T) {
	// Verify all encoded values only use valid base64 characters
	base64Chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	isValidChar := func(c byte) bool {
		for i := 0; i < len(base64Chars); i++ {
			if base64Chars[i] == c {
				return true
			}
		}
		return false
	}

	values := []int{0, 1, -1, 15, -15, 16, -16, 100, -100, 1000, -1000, 10000, -10000}
	for _, v := range values {
		encoded := EncodeVLQ(v)
		for i := 0; i < len(encoded); i++ {
			if !isValidChar(encoded[i]) {
				t.Errorf("EncodeVLQ(%d) = %q contains invalid character %q at position %d",
					v, encoded, string(encoded[i]), i)
			}
		}
	}
}

func TestVLQDecodeSequence(t *testing.T) {
	// Test decoding a sequence of VLQ values
	input := "AAAA" // 0, 0, 0, 0
	expected := []int{0, 0, 0, 0}

	values, err := DecodeVLQSequence(input, 4)
	if err != nil {
		t.Fatalf("DecodeVLQSequence failed: %v", err)
	}

	if len(values) != len(expected) {
		t.Fatalf("DecodeVLQSequence returned %d values, expected %d", len(values), len(expected))
	}

	for i, v := range values {
		if v != expected[i] {
			t.Errorf("values[%d] = %d, expected %d", i, v, expected[i])
		}
	}
}

// ============================================================================
// VLQ Fast Path Tests
// ============================================================================

func TestVLQFastPathSmallPositive(t *testing.T) {
	// Values 0-15 should produce single-digit output (after << 1, fits in 5 bits)
	for v := 0; v <= 15; v++ {
		result := EncodeVLQ(v)
		if len(result) != 1 {
			t.Errorf("EncodeVLQ(%d) = %q (len %d), expected single char", v, result, len(result))
		}
	}
}

func TestVLQFastPathSmallNegative(t *testing.T) {
	// Values -1 to -15 should produce single-digit output
	for v := -1; v >= -15; v-- {
		result := EncodeVLQ(v)
		if len(result) != 1 {
			t.Errorf("EncodeVLQ(%d) = %q (len %d), expected single char", v, result, len(result))
		}
	}
}

func TestVLQFastPathBoundary(t *testing.T) {
	// Test boundary: 15 is last single-digit positive, 16 needs two digits
	if len(EncodeVLQ(15)) != 1 {
		t.Error("EncodeVLQ(15) should be single digit")
	}
	if len(EncodeVLQ(16)) != 2 {
		t.Error("EncodeVLQ(16) should be two digits")
	}
	// Same for negative
	if len(EncodeVLQ(-15)) != 1 {
		t.Error("EncodeVLQ(-15) should be single digit")
	}
	if len(EncodeVLQ(-16)) != 2 {
		t.Error("EncodeVLQ(-16) should be two digits")
	}
}

// Benchmark tests
func BenchmarkVLQEncodeSmall(b *testing.B) {
	// Benchmark small values (should benefit from fast path)
	for i := 0; i < b.N; i++ {
		EncodeVLQ(5)
	}
}

func BenchmarkVLQEncodeLarge(b *testing.B) {
	// Benchmark large values (uses loop)
	for i := 0; i < b.N; i++ {
		EncodeVLQ(1000)
	}
}

func BenchmarkVLQEncode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		EncodeVLQ(1000)
	}
}

func BenchmarkVLQDecode(b *testing.B) {
	encoded := EncodeVLQ(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DecodeVLQ(encoded)
	}
}
