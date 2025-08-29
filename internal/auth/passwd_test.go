package auth_test

import (
	"fmt"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/trice/Chirpy/internal/auth"
)

func TestValidateJWT(t *testing.T) {
    userID := uuid.New()
    tokenSecret := "test-secret"
    expiresIn := time.Hour

    // Generate a valid JWT for testing
    validToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
        Issuer:    "chirpy",
        IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
        ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(expiresIn)),
        Subject:   userID.String(),
    }).SignedString([]byte(tokenSecret))

    // Generate an expired JWT
    expiredToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
        Issuer:    "chirpy",
        IssuedAt:  jwt.NewNumericDate(time.Now().UTC().Add(-2 * time.Hour)),
        ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(-time.Hour)),
        Subject:   userID.String(),
    }).SignedString([]byte(tokenSecret))

    tests := []struct {
        name        string
        tokenString string
        tokenSecret string
        expectedID  uuid.UUID
        expectError bool
    }{
        {
            name:        "Valid JWT",
            tokenString: validToken,
            tokenSecret: tokenSecret,
            expectedID:  userID,
            expectError: false,
        },
        {
            name:        "Expired JWT",
            tokenString: expiredToken,
            tokenSecret: tokenSecret,
            expectedID:  uuid.UUID{}, // Zero value on error
            expectError: true,
        },
        {
            name:        "Invalid token",
            tokenString: "invalid-token",
            tokenSecret: tokenSecret,
            expectedID:  uuid.UUID{},
            expectError: true,
        },
        {
            name:        "Wrong secret",
            tokenString: validToken,
            tokenSecret: "wrong-secret",
            expectedID:  uuid.UUID{},
            expectError: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            gotID, err := auth.ValidateJWT(tt.tokenString, tt.tokenSecret)
            if (err != nil) != tt.expectError {
                t.Errorf("ValidateJWT() error = %v, expectError %v", err, tt.expectError)
                return
            }
            if !tt.expectError && !reflect.DeepEqual(gotID, tt.expectedID) {
                t.Errorf("ValidateJWT() gotID = %v, want %v", gotID, tt.expectedID)
            }
        })
    }
}


func TestMakeJWT(t *testing.T) {
    // Mock os.Getenv by setting an environment variable
    os.Setenv("SECRET", "test-secret")
    defer os.Unsetenv("SECRET") // Clean up after test

    userID := uuid.New()
    expiresIn := time.Hour

    tests := []struct {
        name        string
        userID      uuid.UUID
        tokenSecret string
        expiresIn   time.Duration
        expectError bool
    }{
        {
            name:        "Valid JWT",
            userID:      userID,
            tokenSecret: "test-secret",
            expiresIn:   expiresIn,
            expectError: false,
        },
        {
            name:        "Empty secret",
            userID:      userID,
            tokenSecret: "", // Will use os.Getenv("SECRET")
            expiresIn:   expiresIn,
            expectError: true, // Signing will fail with empty key
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            token, err := auth.MakeJWT(tt.userID, tt.tokenSecret, tt.expiresIn)
            if (err != nil) != tt.expectError {
                t.Errorf("MakeJWT() error = %v, expectError %v", err, tt.expectError)
                return
            }
            if !tt.expectError {
                // Verify the token is valid
                parsedToken, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
                    return []byte(tt.tokenSecret), nil
                })
                if err != nil {
                    t.Errorf("Invalid JWT: %v", err)
                    return
                }
                if !parsedToken.Valid {
                    t.Error("Generated JWT is not valid")
                }
                claims, ok := parsedToken.Claims.(jwt.MapClaims)
                if !ok {
                    t.Error("Failed to parse claims")
                    return
                }
                if claims["sub"] != tt.userID.String() {
                    t.Errorf("JWT subject = %v, want %v", claims["sub"], tt.userID.String())
                }
            }
        })
    }
}


func TestGetBearerToken(t *testing.T) {
    tests := []struct {
        name        string
        token       string
        expectError bool
    }{
        {
            name:        "Valid BearerToken",
            token:      "abc123",
            expectError: false,
        },
        {
            name:        "Empty BearerToken",
            token: "",
            expectError: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            headers := make(http.Header, 1)
            headers.Add("Authorization", fmt.Sprintf("Bearer %s", tt.token))
            token, err := auth.GetBearerToken(headers)
            if tt.expectError == (err == nil) {
               t.Errorf("Expected err but didn't get one")
               return
            }
            if !tt.expectError {
                if token != tt.token {
                    t.Errorf("Token didn't match. Expected: %v, got: %v", tt.token, token)
                    return
                }
            }
        })
    }
}
