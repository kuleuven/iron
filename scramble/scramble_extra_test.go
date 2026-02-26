package scramble

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestGenerateAuthResponse(t *testing.T) {
	challenge := make([]byte, 64)
	for i := range challenge {
		challenge[i] = byte(i)
	}

	result := GenerateAuthResponse(challenge, "testpassword")

	if result == "" {
		t.Fatal("expected non-empty auth response")
	}

	// Should be base64 encoded, so should be a reasonable length
	if len(result) < 10 {
		t.Errorf("auth response unexpectedly short: %q", result)
	}

	// Same input should produce same output
	result2 := GenerateAuthResponse(challenge, "testpassword")
	if result != result2 {
		t.Errorf("expected deterministic response, got %q and %q", result, result2)
	}

	// Different password should produce different output
	result3 := GenerateAuthResponse(challenge, "otherpassword")
	if result == result3 {
		t.Error("expected different response for different password")
	}
}

func TestGenerateAuthResponseEmptyPassword(t *testing.T) {
	challenge := make([]byte, 64)
	result := GenerateAuthResponse(challenge, "")

	if result == "" {
		t.Fatal("expected non-empty auth response even for empty password")
	}
}

func TestScrambleV2(t *testing.T) {
	result := ScrambleV2("newpass", "oldpass", "sig123")

	if result == "" {
		t.Fatal("expected non-empty scrambled result")
	}

	if result == "newpass" {
		t.Error("scrambled result should not equal the input")
	}
}

func TestScrambleV2LongInputs(t *testing.T) {
	longNew := strings.Repeat("a", 200)
	longOld := strings.Repeat("b", 200)
	longSig := strings.Repeat("c", 200)

	// Should not panic with long inputs
	result := ScrambleV2(longNew, longOld, longSig)
	if result == "" {
		t.Fatal("expected non-empty result for long inputs")
	}
}

func TestScrambleDeterministic(t *testing.T) {
	result := Scramble("hello", "mykey", "", false)

	if result == "" {
		t.Fatal("expected non-empty scrambled result")
	}

	if result == "hello" {
		t.Error("scrambled result should differ from input")
	}

	// Deterministic without block chaining
	result2 := Scramble("hello", "mykey", "", false)
	if result != result2 {
		t.Errorf("expected deterministic result without block chaining, got %q and %q", result, result2)
	}
}

func TestScrambleWithPrefix(t *testing.T) {
	result := Scramble("hello", "key", "PREFIX:", false)

	if !strings.HasPrefix(result, "PREFIX:") {
		t.Errorf("expected result to start with 'PREFIX:', got %q", result)
	}
}

func TestScrambleWithBlockChaining(t *testing.T) {
	result1 := Scramble("hello", "key", "", false)
	result2 := Scramble("hello", "key", "", true)

	// Block chaining should produce a different result
	if result1 == result2 {
		t.Error("expected different results with and without block chaining")
	}
}

func TestScrambleDefaultKey(t *testing.T) {
	// Empty key should use default key
	result := Scramble("hello", "", "", false)

	if result == "" {
		t.Fatal("expected non-empty result with default key")
	}

	if result == "hello" {
		t.Error("scrambled result should differ from input")
	}
}

func TestScrambleNonWheelCharacters(t *testing.T) {
	// Characters not in wheel should pass through unchanged
	result := Scramble("@", "key", "", false)

	if result != "@" {
		t.Errorf("expected '@' to pass through unchanged, got %q", result)
	}

	// Mix of wheel and non-wheel characters
	result = Scramble("a@b", "key", "", false)
	if len(result) != 3 {
		t.Errorf("expected length 3, got %d", len(result))
	}

	if result[1] != '@' {
		t.Errorf("expected '@' at position 1, got %c", result[1])
	}
}

func TestGetEncoderRing(t *testing.T) {
	ring := GetEncoderRing("testkey")

	if len(ring) != 64 {
		t.Errorf("expected encoder ring length 64, got %d", len(ring))
	}

	// Should be deterministic
	ring2 := GetEncoderRing("testkey")
	if hex.EncodeToString(ring) != hex.EncodeToString(ring2) {
		t.Error("expected deterministic encoder ring")
	}

	// Different key should produce different ring
	ring3 := GetEncoderRing("otherkey")
	if hex.EncodeToString(ring) == hex.EncodeToString(ring3) {
		t.Error("expected different encoder ring for different key")
	}
}

func TestGetEncoderRingLongKey(t *testing.T) {
	longKey := strings.Repeat("x", 200)
	ring := GetEncoderRing(longKey)

	if len(ring) != 64 {
		t.Errorf("expected encoder ring length 64, got %d", len(ring))
	}
}

func TestGetEncoderRingEmptyKey(t *testing.T) {
	ring := GetEncoderRing("")

	if len(ring) != 64 {
		t.Errorf("expected encoder ring length 64, got %d", len(ring))
	}
}

func TestObfuscateNewPassword(t *testing.T) {
	result := ObfuscateNewPassword("newpass", "oldpass", "signature")

	if result == "" {
		t.Fatal("expected non-empty result")
	}

	if result == "newpass" {
		t.Error("obfuscated result should differ from input")
	}
}

func TestObfuscateNewPasswordLongPassword(t *testing.T) {
	longPass := strings.Repeat("a", 100)
	result := ObfuscateNewPassword(longPass, "oldpass", "sig")

	if result == "" {
		t.Fatal("expected non-empty result for long password")
	}
}

func TestObfuscateNewPasswordShortPadding(t *testing.T) {
	// Password that triggers padding (maxPasswordLength - 10 - len > 15)
	shortPass := "ab"
	result := ObfuscateNewPassword(shortPass, "oldpass", "sig")

	if result == "" {
		t.Fatal("expected non-empty result for short password")
	}
}
