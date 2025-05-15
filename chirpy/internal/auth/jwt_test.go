package auth

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMakeAndValidateJWT(t *testing.T) {
	// Create a test user ID
	userID := uuid.New()
	tokenSecret := "test-secret"
	expiresIn := time.Hour

	// Create a JWT
	tokenString, err := MakeJWT(userID, tokenSecret, expiresIn)
	if err != nil {
		t.Fatalf("Error creating JWT: %v", err)
	}

	// Validate the JWT
	extractedID, err := ValidateJWT(tokenString, tokenSecret)
	if err != nil {
		t.Fatalf("Error validating JWT: %v", err)
	}

	// Check that the extracted ID matches the original
	if extractedID != userID {
		t.Errorf("Expected user ID %v, got %v", userID, extractedID)
	}
}

// func TestValidateJWTWithWrongSecret(t *testing.T) {
// 	// Create a test user ID
// 	userID := uuid.New()
// 	tokenSecret := "test-secret"
// 	wrongSecret := "wrong-secret"
// 	expiresIn := time.Hour

// 	// Create a JWT
// 	tokenString, err := MakeJWT(userID, tokenSecret, expiresIn)

// }

func TestGetBearerToken(t *testing.T) {
	// Create a test header with a Bearer token
	headers := http.Header{}
	headers.Set("Authorization", "Bearer test-token")

	header, err := GetBearerToken(headers)
	fmt.Println(header)
	if err != nil {
		t.Fatalf("Error getting Bearer token: %v", err)
	}
	if header != "test-token" {
		t.Errorf("Expected 'test-token', got '%s'", header)
	}
}
