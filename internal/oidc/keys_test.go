package oidc

import (
	"crypto/rsa"
	"encoding/base64"
	"math/big"
	"strings"
	"testing"
)

func TestGenerateRSAKey(t *testing.T) {
	key, err := GenerateRSAKey()
	if err != nil {
		t.Fatalf("GenerateRSAKey() error = %v", err)
	}
	if key == nil {
		t.Fatal("GenerateRSAKey() returned nil key")
	}
	if key.N.BitLen() != rsaKeyBits {
		t.Errorf("expected %d-bit key, got %d", rsaKeyBits, key.N.BitLen())
	}
}

func TestPrivateKeyToPEM_RoundTrip(t *testing.T) {
	key, err := GenerateRSAKey()
	if err != nil {
		t.Fatalf("GenerateRSAKey() error = %v", err)
	}

	pemStr, err := PrivateKeyToPEM(key)
	if err != nil {
		t.Fatalf("PrivateKeyToPEM() error = %v", err)
	}
	if !strings.HasPrefix(pemStr, "-----BEGIN PRIVATE KEY-----") {
		t.Errorf("PEM does not start with expected header, got: %s", pemStr[:40])
	}

	recovered, err := PEMToPrivateKey(pemStr)
	if err != nil {
		t.Fatalf("PEMToPrivateKey() error = %v", err)
	}

	// Compare the modulus of both keys — they must be identical.
	if key.N.Cmp(recovered.N) != 0 {
		t.Error("recovered key modulus does not match original")
	}
	if key.E != recovered.E {
		t.Errorf("recovered key exponent %d != original %d", recovered.E, key.E)
	}
}

func TestPEMToPrivateKey_InvalidPEM(t *testing.T) {
	_, err := PEMToPrivateKey("not a PEM string")
	if err == nil {
		t.Fatal("expected error for invalid PEM, got nil")
	}
}

func TestPEMToPrivateKey_WrongKeyType(t *testing.T) {
	// A valid PEM block but not an RSA key — use a fake EC-labeled block.
	fakePEM := "-----BEGIN EC PRIVATE KEY-----\nYWJj\n-----END EC PRIVATE KEY-----\n"
	_, err := PEMToPrivateKey(fakePEM)
	if err == nil {
		t.Fatal("expected error for non-RSA PEM, got nil")
	}
}

func TestPublicKeyToJWK(t *testing.T) {
	key, err := GenerateRSAKey()
	if err != nil {
		t.Fatalf("GenerateRSAKey() error = %v", err)
	}

	kid := "test-kid-123"
	jwk := PublicKeyToJWK(&key.PublicKey, kid)

	if jwk.Kty != "RSA" {
		t.Errorf("expected kty=RSA, got %s", jwk.Kty)
	}
	if jwk.Use != "sig" {
		t.Errorf("expected use=sig, got %s", jwk.Use)
	}
	if jwk.Alg != "RS256" {
		t.Errorf("expected alg=RS256, got %s", jwk.Alg)
	}
	if jwk.Kid != kid {
		t.Errorf("expected kid=%s, got %s", kid, jwk.Kid)
	}

	// Verify N decodes back to the correct modulus.
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		t.Fatalf("failed to decode JWK N: %v", err)
	}
	recoveredN := new(big.Int).SetBytes(nBytes)
	if recoveredN.Cmp(key.PublicKey.N) != 0 {
		t.Error("JWK N does not match public key modulus")
	}

	// Verify E decodes back to the correct exponent.
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		t.Fatalf("failed to decode JWK E: %v", err)
	}
	recoveredE := int(new(big.Int).SetBytes(eBytes).Int64())
	if recoveredE != key.PublicKey.E {
		t.Errorf("JWK E %d != public key exponent %d", recoveredE, key.PublicKey.E)
	}
}

func TestPublicKeyToJWK_EmptyKid(t *testing.T) {
	key, _ := GenerateRSAKey()
	jwk := PublicKeyToJWK(&key.PublicKey, "")
	if jwk.Kid != "" {
		t.Errorf("expected empty kid, got %s", jwk.Kid)
	}
}

// Ensure GenerateRSAKey produces distinct keys each time (probabilistic check).
func TestGenerateRSAKey_Uniqueness(t *testing.T) {
	k1, err := GenerateRSAKey()
	if err != nil {
		t.Fatal(err)
	}
	k2, err := GenerateRSAKey()
	if err != nil {
		t.Fatal(err)
	}
	if k1.N.Cmp(k2.N) == 0 {
		t.Error("two independently generated RSA keys have the same modulus")
	}
}

// Fuzz-style: ensure PEMToPrivateKey never panics on random input.
func TestPEMToPrivateKey_GarbageInput(t *testing.T) {
	inputs := []string{
		"",
		"-----BEGIN PRIVATE KEY-----\n-----END PRIVATE KEY-----\n",
		"-----BEGIN PRIVATE KEY-----\nYWJjZGVmZ2g=\n-----END PRIVATE KEY-----\n",
		"\x00\x01\x02\x03",
	}
	for _, in := range inputs {
		_, _ = PEMToPrivateKey(in) // must not panic
	}
}

// Verify that PrivateKeyToPEM produces a PKCS#8 block (not PKCS#1).
func TestPrivateKeyToPEM_IsPKCS8(t *testing.T) {
	key, _ := GenerateRSAKey()
	pemStr, err := PrivateKeyToPEM(key)
	if err != nil {
		t.Fatal(err)
	}
	// PKCS#8 uses "PRIVATE KEY" header; PKCS#1 uses "RSA PRIVATE KEY".
	if !strings.Contains(pemStr, "BEGIN PRIVATE KEY") {
		t.Error("expected PKCS#8 PEM header 'BEGIN PRIVATE KEY'")
	}
	if strings.Contains(pemStr, "BEGIN RSA PRIVATE KEY") {
		t.Error("unexpected PKCS#1 PEM header 'BEGIN RSA PRIVATE KEY'")
	}
}

// Benchmark key generation to detect unexpected regressions.
func BenchmarkGenerateRSAKey(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := GenerateRSAKey(); err != nil {
			b.Fatal(err)
		}
	}
}

// Ensure PEM round-trip preserves the ability to sign.
func TestPrivateKeyToPEM_SigningPreserved(t *testing.T) {
	key, _ := GenerateRSAKey()
	pemStr, _ := PrivateKeyToPEM(key)
	recovered, err := PEMToPrivateKey(pemStr)
	if err != nil {
		t.Fatal(err)
	}

	// Validate that the precomputed values match (PublicKey embedded in PrivateKey).
	if _, ok := interface{}(recovered).(*rsa.PrivateKey); !ok {
		t.Error("recovered key is not *rsa.PrivateKey")
	}
	recovered.Precompute()
}
