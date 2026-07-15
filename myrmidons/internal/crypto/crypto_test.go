package crypto

import (
	"testing"
)

// TestValidateXMRAddress tests Monero address validation
func TestValidateXMRAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		valid   bool
	}{
		// Valid addresses (mock)
		{"valid standard", "44AFFq5kSiGBoZ4N9wY48uNh7Yj6JJJb7h6jfVYS3xSqtdCMRbp8VMqDGSR2mKp7E7M5e8vPJUWu1V5mY9v8Z6fQ3kR5d", true},

		// Invalid addresses
		{"too short", "44AFFq5kSiGBoZ4N9wY48", false},
		{"too long", "44AFFq5kSiGBoZ4N9wY48uNh7Yj6JJJb7h6jfVYS3xSqtdCMRbp8VMqDGSR2mKp7E7M5e8vPJUWu1V5mY9v8Z6fQ3kR5dEXTRAA", false},
		{"wrong start", "54AFFq5kSiGBoZ4N9wY48uNh7Yj6JJJb7h6jfVYS3xSqtdCMRbp8VMqDGSR2mKp7E7M5e8vPJUWu1V5mY9v8Z6fQ3kR5d", false},
		{"invalid char", "44AFFq5kSiGBoZ4N9wY48uNh7Yj6JJJb7h6jfVYS3xSqtdCMRbp8VMqDGSR2mKp7E7M5e8vPJUWu1V5mY9v8Z6fQ3kR5", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateXMRAddress(tt.address)
			if result != tt.valid {
				t.Errorf("ValidateXMRAddress(%q) = %v, want %v",
					tt.address, result, tt.valid)
			}
		})
	}
}

// TestConstantTimeCompare tests constant-time comparison
func TestConstantTimeCompare(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected bool
	}{
		{"equal strings", "hello", "hello", true},
		{"different strings", "hello", "world", false},
		{"different length", "hello", "hello!", false},
		{"empty both", "", "", true},
		{"empty a", "", "hello", false},
		{"empty b", "hello", "", false},
		{"long equal", "abcdefghijklmnop", "abcdefghijklmnop", true},
		{"long different", "abcdefghijklmnop", "abcdefghijklmnoo", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConstantTimeCompare(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("ConstantTimeCompare(%q, %q) = %v, want %v",
					tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

// TestGenerateRandomBytes tests random byte generation
func TestGenerateRandomBytes(t *testing.T) {
	// Test different sizes
	sizes := []int{16, 32, 64, 128}

	for _, size := range sizes {
		t.Run("", func(t *testing.T) {
			bytes, err := GenerateRandomBytes(size)
			if err != nil {
				t.Errorf("GenerateRandomBytes(%d) returned error: %v", size, err)
			}
			if len(bytes) != size {
				t.Errorf("GenerateRandomBytes(%d) returned %d bytes", size, len(bytes))
			}

			// Generate another and ensure they're different
			bytes2, _ := GenerateRandomBytes(size)
			same := true
			for i := 0; i < size; i++ {
				if bytes[i] != bytes2[i] {
					same = false
					break
				}
			}
			if same {
				t.Error("Two calls to GenerateRandomBytes returned same values")
			}
		})
	}
}

// TestGenerateRandomHex tests random hex generation
func TestGenerateRandomHex(t *testing.T) {
	hex1, err := GenerateRandomHex(32)
	if err != nil {
		t.Errorf("GenerateRandomHex(32) returned error: %v", err)
	}
	if len(hex1) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("GenerateRandomHex(32) returned %d chars, want 64", len(hex1))
	}
}

// TestEncryptDecryptAES tests AES encryption/decryption
func TestEncryptDecryptAES(t *testing.T) {
	// Generate a 32-byte key
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintexts := []string{
		"hello world",
		"test message with special chars: !@#$%",
		"A" + string(make([]byte, 1000)), // Long message
		"日本語テスト", // Unicode
		"",
	}

	for _, plaintext := range plaintexts {
		t.Run("", func(t *testing.T) {
			ciphertext, nonce, err := EncryptAES(key, []byte(plaintext))
			if err != nil {
				t.Errorf("EncryptAES failed: %v", err)
				return
			}

			if len(nonce) != 12 {
				t.Errorf("Nonce should be 12 bytes, got %d", len(nonce))
			}

			decrypted, err := DecryptAES(key, nonce, ciphertext)
			if err != nil {
				t.Errorf("DecryptAES failed: %v", err)
				return
			}

			if string(decrypted) != plaintext {
				t.Errorf("Decrypted text doesn't match: got %q, want %q",
					string(decrypted), plaintext)
			}
		})
	}
}

// TestAESWrongKey tests that wrong key fails decryption
func TestAESWrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key2[0] = 1 // Different key

	for i := range key1 {
		key1[i] = byte(i)
		key2[i] = byte(i)
	}

	ciphertext, nonce, _ := EncryptAES(key1, []byte("secret message"))
	_, err := DecryptAES(key2, nonce, ciphertext)
	if err == nil {
		t.Error("Decryption with wrong key should fail")
	}
}

// TestAmountConversions tests XMR/piconero conversions
func TestAmountConversions(t *testing.T) {
	tests := []struct {
		piconero int64
		xmr      float64
	}{
		{1000000000000, 0.001},         // 0.001 XMR
		{10000000000, 0.00001},         // 0.00001 XMR
		{123456789012, 0.000123456789}, // Precise
		{0, 0},                         // Zero
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := AmountFromPiconero(tt.piconero)
			if result != tt.xmr {
				t.Errorf("AmountFromPiconero(%d) = %f, want %f",
					tt.piconero, result, tt.xmr)
			}

			back := AmountToPiconero(tt.xmr)
			if back != tt.piconero {
				t.Errorf("AmountToPiconero(%f) = %d, want %d",
					tt.xmr, back, tt.piconero)
			}
		})
	}
}

// TestFormatXMR tests XMR formatting
func TestFormatXMR(t *testing.T) {
	tests := []struct {
		piconero int64
		formatted string
	}{
		{1000000000000, "0.001000000000"},
		{0, "0.000000000000"},
		{1, "0.000000000001"},
	}

	for _, tt := range tests {
		result := FormatXMR(tt.piconero)
		if result != tt.formatted {
			t.Errorf("FormatXMR(%d) = %q, want %q",
				tt.piconero, result, tt.formatted)
		}
	}
}
