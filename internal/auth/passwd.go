package auth

import "golang.org/x/crypto/bcrypt"

func HashPassword(password string) (string, error) {
    hp, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
    return string(hp), err
}

func CheckPasswordHash(password, hash string) error {
    return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
