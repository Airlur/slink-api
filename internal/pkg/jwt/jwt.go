package jwt

import (
	"context"
	"fmt"
	"strings"

	"slink-api/internal/model"
	"slink-api/internal/pkg/config"
	bizErrors "slink-api/internal/pkg/errors"
	"slink-api/internal/pkg/logger"
	"slink-api/internal/pkg/redis"
	"slink-api/internal/pkg/response"
	"time"
	
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

// 【核心修改点】将Token类型定义为包内常量
const (
	AccessTokenType  = "access"
	RefreshTokenTpye = "refresh"
)

const (
	TokenActivePrefix = "token:refresh:" // Redis key前缀，改为存储RefreshToken
)

// AccessTokenClaims 定义了 Access Token 的载荷
type AccessTokenClaims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     int    `json:"role"`
	Type     string `json:"type"` // 令牌类型
	jwt.RegisteredClaims
}

// RefreshTokenClaims 定义了 Refresh Token 的载荷
type RefreshTokenClaims struct {
	UserID uint   `json:"user_id"`
	Type   string `json:"type"` // 令牌类型
	jwt.RegisteredClaims
}

// GenerateTokens 生成 Access Token 和 Refresh Token
func GenerateTokens(ctx context.Context, user *model.User) (accessToken, refreshToken string, err error) {
	// --- 创建 Access Token (短生命周期) ---
	accessExpireTime := time.Now().Add(time.Duration(config.GlobalConfig.JWT.AccessExpireMinutes) * time.Minute)
	accessClaims := AccessTokenClaims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		Type:     AccessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessExpireTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    config.GlobalConfig.JWT.Issuer,
		},
	}
	accessToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(config.GlobalConfig.JWT.Secret))
	if err != nil {
		return "", "", err
	}

	// --- 创建 Refresh Token (长生命周期) ---
	refreshExpireTime := time.Now().Add(time.Duration(config.GlobalConfig.JWT.RefreshExpireHours) * time.Hour)
	refreshClaims := RefreshTokenClaims{
		UserID:   user.ID,
		Type:   RefreshTokenTpye, // 【核心修改点】使用常量
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(refreshExpireTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    config.GlobalConfig.JWT.Issuer,
			ID:        uuid.NewString(),
		},
	}
	refreshToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(config.GlobalConfig.JWT.Secret))
	if err != nil {
		return "", "", err
	}

	// --- 将 Refresh Token 存储到 Redis ---
	key := fmt.Sprintf("%s%d", TokenActivePrefix, user.ID)
	if err = redis.Set(ctx, key, refreshToken, time.Duration(config.GlobalConfig.JWT.RefreshExpireHours)*time.Hour); err != nil {
		logger.Error("存储RefreshToken失败", "error", err)
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// ParseAccessToken 专门用于解析 Access Token
func ParseAccessToken(tokenString string) (*AccessTokenClaims, error) {
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	token, err := jwt.ParseWithClaims(tokenString, &AccessTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.GlobalConfig.JWT.Secret), nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*AccessTokenClaims); ok && token.Valid {
		if claims.Type != AccessTokenType {
			return nil, bizErrors.New(response.InvalidToken, "无效的 Token 类型")
		}
		return claims, nil
	}
	return nil, jwt.ErrSignatureInvalid
}

// ParseRefreshToken 专门用于解析 Refresh Token
func ParseRefreshToken(tokenString string) (*RefreshTokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &RefreshTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.GlobalConfig.JWT.Secret), nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*RefreshTokenClaims); ok && token.Valid {
		if claims.Type != RefreshTokenTpye {
			return nil, bizErrors.New(response.InvalidToken, "无效的 Token 类型")
		}
		return claims, nil
	}
	return nil, jwt.ErrSignatureInvalid
}

// IsRefreshTokenActive 检查 Refresh Token 是否在Redis中仍然活跃
func IsRefreshTokenActive(ctx context.Context, tokenString string, userID uint) bool {
	key := fmt.Sprintf("%s%d", TokenActivePrefix, userID)
	activeToken, err := redis.Get(ctx, key)
	if err != nil || activeToken != tokenString {
		return false
	}
	return true
}

// InvalidateToken 使 Refresh Token 失效（用于登出）
func InvalidateToken(ctx context.Context, tokenString string) error {
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	claims, err := ParseRefreshToken(tokenString)
	if err != nil {
		// 如果token本身就无法解析，可以认为它已“失效”
		return nil
	}
	key := fmt.Sprintf("%s%d", TokenActivePrefix, claims.UserID)
	if err := redis.Del(ctx, key); err != nil {
		logger.Error("从Redis删除RefreshToken失败", "error", err, "key", key)
		return err
	}
	return nil
}

type UserInfo struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Role     int    `json:"role"`
}

func GetUserInfo(c *gin.Context) *UserInfo {
	value, exists := c.Get("user_info")
	if !exists {
		return nil // 不存在就返回 nil
	}
	userInfo, ok := value.(*UserInfo)
    if !ok {
        return nil // 类型不对返回 nil
    }
	return userInfo
}
