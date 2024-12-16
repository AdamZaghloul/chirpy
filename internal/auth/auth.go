package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hashed), nil
}

func CheckPasswordHash(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))

}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.StandardClaims{
		Issuer:    "chirpy",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(expiresIn).Unix(),
		Subject:   userID.String(),
	})

	signed, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}

	return signed, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	claims := jwt.StandardClaims{}

	keyFunc := func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Return the secret key for HMAC
		return []byte(tokenSecret), nil
	}

	_, err := jwt.ParseWithClaims(tokenString, &claims, keyFunc)
	if err != nil {
		return uuid.UUID{}, err
	}

	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.UUID{}, err
	}

	return id, nil
}

func GetToken(headers http.Header, prefix string) (string, error) {
	auth := headers.Get("Authorization")
	if auth == "" {
		return "", errors.New("no authorization header")
	}

	if !strings.HasPrefix(auth, prefix) {
		return "", errors.New("invalid authorization header")
	}

	token := strings.TrimPrefix(auth, prefix)
	token = strings.TrimSpace(token)

	if token == "" {
		return "", errors.New("authorization token is empty")
	}

	return token, nil
}

func MakeRefreshToken() (string, error) {
	len := 32
	token := make([]byte, len)
	_, err := rand.Read(token)
	if err != nil {
		return "", err
	}

	hexToken := hex.EncodeToString(token)

	return hexToken, nil
}
