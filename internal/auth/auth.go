package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Claims struct {
	UserID   int64  `json:"uid"`
	Username string `json:"username"`
	Type     string `json:"typ"` // access | refresh
	jwt.RegisteredClaims
}

type Auth struct {
	Secret        []byte
	AccessExpire  time.Duration
	RefreshExpire time.Duration
}

func New(secret string, accessMin int, refreshDays int) *Auth {
	access := time.Duration(accessMin) * time.Minute
	if access <= 0 {
		access = 15 * time.Minute
	}
	refresh := time.Duration(refreshDays) * 24 * time.Hour
	if refresh <= 0 {
		refresh = 30 * 24 * time.Hour
	}
	return &Auth{Secret: []byte(secret), AccessExpire: access, RefreshExpire: refresh}
}

func (a *Auth) TokenPair(userID int64, username string) (access, refresh string, err error) {
	now := time.Now()
	access, err = a.sign(Claims{UserID: userID, Username: username, Type: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: username, IssuedAt: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(a.AccessExpire)),
		}})
	if err != nil {
		return
	}
	refresh, err = a.sign(Claims{UserID: userID, Username: username, Type: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: username, IssuedAt: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(a.RefreshExpire)),
		}})
	return
}

func (a *Auth) sign(c Claims) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(a.Secret)
}

func (a *Auth) Parse(tokenStr string, typ string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return a.Secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid || claims.Type != typ {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

func VerifyPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// HashToken derives a SHA-256 hex digest for PAT lookup.
func HashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func RandomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func CheckPasswordPolicy(pw string) bool {
	if len(pw) < 8 {
		return false
	}
	return true
}

func NormalizeUsername(u string) string {
	return strings.TrimSpace(u)
}
