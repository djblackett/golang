package auth

import (
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"crypto/rand"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func CheckPasswordHash(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	// var key *ecdsa.PrivateKey
	var t *jwt.Token
	var s string

	t = jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
		Subject:   userID.String(),
	})

	s, err := t.SignedString([]byte(tokenSecret))
	return s, err
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {

	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(tokenSecret), nil
	}, jwt.WithLeeway(5*time.Second))
	if err != nil {
		return uuid.Nil, err
	}
	if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && token.Valid {
		id := claims.Subject
		userID, err := uuid.Parse(id)
		if err != nil {
			return uuid.Nil, err
		}
		return userID, nil
	}
	return uuid.Nil, jwt.ErrTokenInvalidClaims
}

func GetBearerToken(headers http.Header) (string, error) {
	header := headers.Get("Authorization")
	if len(header) < 7 {
		return "", jwt.ErrTokenMalformed
	}

	token := header[7:]
	if len(token) == 0 {
		return "", jwt.ErrTokenMalformed
	}
	return token, nil
}

func MakeRefreshToken() (string, error) {
	token := make([]byte, 32)
	rand.Read(token)
	return hex.EncodeToString(token), nil
}

func GetAPIKey(headers http.Header) (string, error) {
	header := headers.Get("Authorization")
	if len(header) < 6 {
		return "", errors.New("invalid api key")
	}
	apiKey := header[6:]
	return apiKey, nil
}
