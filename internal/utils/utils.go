package utils

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

// KeyPair represents a WireGuard key pair
type KeyPair struct {
	Private string
	Public  string
}

// GenerateWireGuardKeyPair generates a WireGuard key pair
func GenerateWireGuardKeyPair() (*KeyPair, error) {
	privBytes := make([]byte, 32)
	if _, err := rand.Read(privBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Clamp the private key according to WireGuard specification
	privBytes[0] &= 248
	privBytes[31] &= 127
	privBytes[31] |= 64

	// Generate public key
	pubBytes, err := curve25519.X25519(privBytes, curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("failed to generate public key: %w", err)
	}

	return &KeyPair{
		Private: base64.StdEncoding.EncodeToString(privBytes),
		Public:  base64.StdEncoding.EncodeToString(pubBytes),
	}, nil
}

func Filter[T any](ss []T, test func(T) bool) (ret []T) {
	for _, s := range ss {
		if test(s) {
			ret = append(ret, s)
		}
	}
	return
}

func EqualStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
