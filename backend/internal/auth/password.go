package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/scrypt"
)

func HashPassword(password, appSecret string) (string, error) {
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(password))
	hmacHex := hex.EncodeToString(mac.Sum(nil))

	saltBytes := make([]byte, 16)
	if _, err := rand.Read(saltBytes); err != nil {
		return "", err
	}
	salt := hex.EncodeToString(saltBytes)

	derivedKey, err := scrypt.Key([]byte(hmacHex), []byte(salt), 16384, 8, 1, 64)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%s", salt, hex.EncodeToString(derivedKey)), nil
}

func VerifyPassword(password, storedHash, appSecret string) bool {
	if strings.HasPrefix(storedHash, "$2a$") || strings.HasPrefix(storedHash, "$2b$") {
		return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)) == nil
	}

	parts := strings.Split(storedHash, ":")
	if len(parts) != 2 {
		return false
	}

	salt := parts[0]
	expectedHash := parts[1]

	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(password))
	hmacHex := hex.EncodeToString(mac.Sum(nil))

	derivedKey, err := scrypt.Key([]byte(hmacHex), []byte(salt), 16384, 8, 1, 64)
	if err != nil {
		return false
	}

	calculatedHash := hex.EncodeToString(derivedKey)
	return subtle.ConstantTimeCompare([]byte(calculatedHash), []byte(expectedHash)) == 1
}
