package auth

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestValidateJWT(t *testing.T) {
	id := uuid.New()

	tokenString, err := MakeJWT(id, os.Getenv("TOKEN_SECRET"), time.Second)
	if err != nil {
		t.Fatalf("Error making token")
	}

	_, err = ValidateJWT(tokenString, os.Getenv("TOKEN_SECRET"))

	if err != nil {
		t.Fatalf("Failed to validate valid token: %s", err.Error())
	}

}

func TestValidateJWTWrongSecret(t *testing.T) {
	id := uuid.New()

	tokenString, err := MakeJWT(id, "TEST_SECRET", time.Second)
	if err != nil {
		t.Fatalf("Error making token")
	}

	_, err = ValidateJWT(tokenString, os.Getenv("TOKEN_SECRET"))

	if err == nil {
		t.Fatalf("Validated token signed with wrong secret")
	}

}

func TestValidateJWTExpired(t *testing.T) {
	id := uuid.New()

	tokenString, err := MakeJWT(id, os.Getenv("TOKEN_SECRET"), time.Millisecond)
	if err != nil {
		t.Fatalf("Error making token")
	}

	time.Sleep(time.Second)

	_, err = ValidateJWT(tokenString, os.Getenv("TOKEN_SECRET"))

	if err == nil {
		t.Fatalf("Validated expired token")
	}

}

func TestGetBearerToken(t *testing.T) {
	headers := http.Header{}
	headers.Add("Authorization", "Bearer TOKEN")

	token, err := GetBearerToken(headers)
	if err != nil {
		t.Fatalf("%s", err)
	}

	if token != "TOKEN" {
		t.Fatalf("token incorrectly parsed")
	}
}

func TestGetBearerTokenNoHeader(t *testing.T) {
	headers := http.Header{}

	_, err := GetBearerToken(headers)
	if err == nil {
		t.Fatalf("Parsed token from headers with no auth header")
	}
}
