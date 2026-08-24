// Package auth 提供密码哈希（Argon2id）与 JWT 签发/校验。
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
)

// ============ Argon2id 密码哈希（PHC 字符串格式） ============

const (
	argonTime    uint32 = 1
	argonMemory  uint32 = 64 * 1024 // 64 MB
	argonThreads uint8  = 4
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16
)

// ErrInvalidPasswordHash 密码哈希格式非法。
var ErrInvalidPasswordHash = errors.New("invalid password hash format")

// HashPassword 使用 Argon2id 生成 PHC 格式密码哈希。
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	enc := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key))
	return enc, nil
}

// VerifyPassword 校验明文密码与 PHC 哈希是否匹配。
func VerifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	// $argon2id$v=19$m=65536,t=1,p=4$<salt>$<hash>
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrInvalidPasswordHash
	}
	var version uint32
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("parse version: %w", err)
	}
	var mem uint32
	var iters uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &iters, &threads); err != nil {
		return false, fmt.Errorf("parse params: %w", err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode salt: %w", err)
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode hash: %w", err)
	}
	got := argon2.IDKey([]byte(password), salt, iters, mem, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// ============ JWT ============

// 令牌受众（aud）。
const (
	AudienceInternal  = "internal"
	AudienceCandidate = "candidate"
)

// Claims 内部端 JWT 载荷（sub/role/dep_id/name）。
type Claims struct {
	Role string `json:"role,omitempty"`
	// DepID 部门 id，可为空（admin/hr 无部门）。
	DepID *string `json:"dep_id,omitempty"`
	// Name 用户姓名（OA 流转时间线展示操作人）。
	Name string `json:"name,omitempty"`
	jwt.RegisteredClaims
}

// Sign 签发 HS256 JWT（aud=internal）。
func Sign(secret string, ttl time.Duration, userID, role string, depID *string, name string) (string, error) {
	return sign(secret, ttl, AudienceInternal, userID, role, depID, name)
}

// SignCandidate 签发求职者 JWT（aud=candidate）。
func SignCandidate(secret string, ttl time.Duration, candidateID string) (string, error) {
	return sign(secret, ttl, AudienceCandidate, candidateID, "", nil, "")
}

func sign(secret string, ttl time.Duration, aud, sub, role string, depID *string, name string) (string, error) {
	now := time.Now()
	claims := Claims{
		Role:  role,
		DepID: depID,
		Name:  name,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   sub,
			Audience:  jwt.ClaimStrings{aud},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// Parse 校验并解析 JWT，返回 Claims。
func Parse(secret, tokenString string, allowedAudience string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	}, jwt.WithAudience(allowedAudience), jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	return claims, nil
}
