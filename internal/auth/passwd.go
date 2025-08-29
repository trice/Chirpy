package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
    hp, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
    return string(hp), err
}

func CheckPasswordHash(password, hash string) error {
    return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
    if len(tokenSecret) == 0 {
        return "", fmt.Errorf("Invalid tokenSecret")
    }

    now := time.Now().UTC()
    issue := jwt.NewNumericDate(now)
    expire := jwt.NewNumericDate(now.Add(expiresIn).UTC())

    claims := jwt.RegisteredClaims {
        Issuer: "chirpy",
        IssuedAt: issue,
        ExpiresAt: expire,
        Subject: userID.String(),
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    theJwt, err := token.SignedString([]byte(tokenSecret))
    return theJwt, err
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
    if len(tokenSecret) == 0 || tokenSecret == "" {
        return uuid.UUID{}, fmt.Errorf("Invalid tokenSecret")
    }

    if len(tokenString) == 0 || tokenString == "" {
        return uuid.UUID{}, fmt.Errorf("Invalid tokenString")
    }

    holder := jwt.RegisteredClaims{}

    token, err := jwt.ParseWithClaims(tokenString, &holder, func(t *jwt.Token) (any, error) {
        return []byte(tokenSecret), nil
    }, jwt.WithLeeway(5 * time.Second))
    if err != nil {
        return uuid.UUID{}, fmt.Errorf("Failed to parse tokenString")
    }

    userId, err := token.Claims.GetSubject()
    if err != nil {
        return uuid.UUID{}, err
    }
    returnUuid, err := uuid.Parse(userId)
    if err != nil {
        return uuid.UUID{}, err
    }

    return returnUuid, nil
}

func GetBearerToken(headers http.Header) (string, error) {
    bt := headers.Values("Authorization")[0][7:]
    if len(bt) == 0 {
        return "", fmt.Errorf("No token found")
    }
    return bt, nil
}

func MakeRefreshToken() (string, error) {
    tokenBuf := make([]byte, 32)

    i, err := rand.Read(tokenBuf)
    if i < 32 || err != nil {
        return "", err
    }

    refreshToken := hex.EncodeToString(tokenBuf)
    return refreshToken, nil
}
