// Package crypto provides bcrypt password hashing utilities.
package crypto

import "golang.org/x/crypto/bcrypt"

const cost = 12

// HashPassword returns the bcrypt hash of the given password.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	return string(b), err
}

// VerifyPassword checks whether plain matches the bcrypt hash.
func VerifyPassword(plain, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
