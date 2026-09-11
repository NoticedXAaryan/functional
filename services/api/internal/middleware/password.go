package middleware

import "golang.org/x/crypto/bcrypt"

// verifyPassword checks a bcrypt hash against a plaintext password.
func verifyPassword(hash, plaintext string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext))
}

// HashPassword generates a bcrypt hash for a plaintext password.
// Used in the seed command only — never called on request paths.
func HashPassword(plaintext string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plaintext), 12)
	return string(b), err
}
