package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/argon2"
)

// --- Password hashing (Argon2id) ---

type ArgonParams struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

func DefaultArgonParams() ArgonParams {
	return ArgonParams{Memory: 64 * 1024, Iterations: 3, Parallelism: 2, SaltLength: 16, KeyLength: 32}
}

// HashPassword returns a PHC-like string $argon2id$v=19$m=..,t=..,p=..$salt$hash.
func HashPassword(password string) (string, error) {
	p := DefaultArgonParams()
	salt := make([]byte, p.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)
	return encodeHash(p, salt, hash), nil
}

// VerifyPassword checks password against an encoded hash.
func VerifyPassword(encoded, password string) bool {
	p, salt, hash, err := decodeHash(encoded)
	if err != nil {
		return false
	}
	computed := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)
	return subtleEqual(computed, hash)
}

// --- JWT ---

// Claims carried in access tokens.
type Claims struct {
	UserID  uint   `json:"uid"`
	IsAdmin bool   `json:"admin"`
	jwt.RegisteredClaims
}

// IssueJWT mints a short-lived access token.
func IssueJWT(secret string, userID uint, isAdmin bool, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID:  userID,
		IsAdmin: isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "mine-panel",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ParseJWT validates and returns claims.
func ParseJWT(secret, token string) (*Claims, error) {
	t, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	c, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, errors.New("invalid token")
	}
	return c, nil
}

// --- TOTP 2FA ---

// GenerateTOTPSecret creates a new TOTP secret + provisioning URI.
func GenerateTOTPSecret(account string) (secret, uri string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "MINE",
		AccountName: account,
		Period:      30,
		Digits:      6,
	})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

// VerifyTOTP checks a 6-digit code.
func VerifyTOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}

// --- AES-256-GCM secrets at rest ---

// EncryptSecret encrypts plaintext with a 32-byte key, returns base64 nonce|ciphertext.
func EncryptSecret(key32, plaintext string) (string, error) {
	if len(key32) < 32 {
		return "", errors.New("encryption key must be >= 32 bytes")
	}
	key := []byte(key32)[:32]
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// DecryptSecret reverses EncryptSecret.
func DecryptSecret(key32, encoded string) (string, error) {
	if len(key32) < 32 {
		return "", errors.New("encryption key must be >= 32 bytes")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	key := []byte(key32)[:32]
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return "", errors.New("ciphertext too short")
	}
	pt, err := gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}
