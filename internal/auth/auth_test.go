package auth

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHashPassword(t *testing.T) {
	password := "test_password_123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if hash == "" {
		t.Error("HashPassword returned empty hash")
	}

	// Test that the same password hashed twice produces different hashes
	// (argon2id includes random salt)
	hash2, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed on second call: %v", err)
	}

	if hash == hash2 {
		t.Error("Two hashes of the same password should be different due to random salt")
	}
}

func TestCheckPasswordHash_ValidPassword(t *testing.T) {
	password := "correct_password"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword setup failed: %v", err)
	}

	match, err := CheckPasswordHash(password, hash)
	if err != nil {
		t.Fatalf("CheckPasswordHash failed: %v", err)
	}

	if !match {
		t.Error("CheckPasswordHash should return true for matching password")
	}
}

func TestCheckPasswordHash_InvalidPassword(t *testing.T) {
	correctPassword := "correct_password"
	wrongPassword := "wrong_password"

	hash, err := HashPassword(correctPassword)
	if err != nil {
		t.Fatalf("HashPassword setup failed: %v", err)
	}

	match, err := CheckPasswordHash(wrongPassword, hash)
	if err != nil {
		t.Fatalf("CheckPasswordHash failed: %v", err)
	}

	if match {
		t.Error("CheckPasswordHash should return false for non-matching password")
	}
}

func TestCheckPasswordHash_EmptyPassword(t *testing.T) {
	password := "some_password"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword setup failed: %v", err)
	}

	match, err := CheckPasswordHash("", hash)
	if err != nil {
		t.Fatalf("CheckPasswordHash failed: %v", err)
	}

	if match {
		t.Error("CheckPasswordHash should return false for empty password")
	}
}

func TestMakeJWT(t *testing.T) {
	userID := uuid.New()
	tokenSecret := "test_secret_key"
	expiresIn := 1 * time.Hour

	token, err := MakeJWT(userID, tokenSecret, expiresIn)
	if err != nil {
		t.Fatalf("MakeJWT failed: %v", err)
	}

	if token == "" {
		t.Error("MakeJWT returned empty token")
	}

	// Validate the token we just created
	parsedUserID, err := ValidateJWT(token, tokenSecret)
	if err != nil {
		t.Fatalf("ValidateJWT failed: %v", err)
	}

	if parsedUserID != userID {
		t.Errorf("ValidateJWT returned different userID. Expected %v, got %v", userID, parsedUserID)
	}
}

func TestMakeJWT_DifferentTokensForDifferentUsers(t *testing.T) {
	userID1 := uuid.New()
	userID2 := uuid.New()
	tokenSecret := "test_secret_key"
	expiresIn := 1 * time.Hour

	token1, err := MakeJWT(userID1, tokenSecret, expiresIn)
	if err != nil {
		t.Fatalf("MakeJWT failed for user1: %v", err)
	}

	token2, err := MakeJWT(userID2, tokenSecret, expiresIn)
	if err != nil {
		t.Fatalf("MakeJWT failed for user2: %v", err)
	}

	if token1 == token2 {
		t.Error("Different users should produce different tokens")
	}
}

func TestValidateJWT_InvalidToken(t *testing.T) {
	tokenSecret := "test_secret_key"

	_, err := ValidateJWT("invalid_token", tokenSecret)
	if err == nil {
		t.Error("ValidateJWT should return error for invalid token")
	}
}

func TestValidateJWT_WrongSecret(t *testing.T) {
	userID := uuid.New()
	tokenSecret := "test_secret_key"
	wrongSecret := "wrong_secret_key"
	expiresIn := 1 * time.Hour

	token, err := MakeJWT(userID, tokenSecret, expiresIn)
	if err != nil {
		t.Fatalf("MakeJWT failed: %v", err)
	}

	_, err = ValidateJWT(token, wrongSecret)
	if err == nil {
		t.Error("ValidateJWT should return error when using wrong secret")
	}
}

func TestGetAuthTokenFromHeader(t *testing.T) {
	headers := http.Header{}
	token := "thisisatoken"
	headers.Add("Authorization", "Bearer "+token)

	rsToken, err := GetBearerToken(headers)

	if err != nil {
		t.Fatalf("error getting token %v", err)
	}

	if rsToken != token {
		t.Errorf("Expected: %s, Returned: %s", token, rsToken)
	}

}
