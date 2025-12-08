package middleware

import (
	"short-link/internal/pkg/jwt"
	"short-link/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Fail(c, response.Unauthorized, "")
			c.Abort()
			return
		}

		// parts := strings.SplitN(authHeader, " ", 2)
		// if !(len(parts) == 2 && parts[0] == "Bearer") {
		// 	response.Fail(c, response.InvalidToken, "")
		// 	c.Abort()
		// 	return
		// }
		// tokenString := parts[1]

		// Auth中间件只负责验证AccessToken的合法性和有效期
		claims, err := jwt.ParseAccessToken(authHeader)
		if err != nil {
			// ParseAccessToken 内部会检查签名、有效期和类型
			response.Fail(c, response.InvalidToken, "Token 无效或已过期")
			c.Abort()
			return
		}

		// 存储用户信息到context
		userInfo := &jwt.UserInfo{
			ID:       claims.UserID,
			Username: claims.Username,
			Role:     claims.Role,
		}
		c.Set("user_info", userInfo)
		c.Next()
	}
}

// AuthOptional 创建一个可选认证的中间件。
// - 如果请求头中带有有效的 Authorization token，则解析用户信息并存入 context。
// - 如果请求头中没有 Authorization token，则直接放行，视为游客。
// - 如果请求头中带有无效/过期的 token，则返回错误。
func AuthOptional() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		// Case 1: 没有 Token，视为游客
		if authHeader == "" {
			c.Next()
			return
		}

		// Case 2: 有 Token，必须严格校验。执行与Auth()中间件完全相同的验证逻辑
		// parts := strings.SplitN(authHeader, " ", 2)
		// if !(len(parts) == 2 && parts[0] == "Bearer") {
		// 	// 如果有 Authorization 头但格式不正确，也视为无效
		// 	response.Fail(c, response.InvalidToken, "无效的token格式")
		// 	c.Abort()
		// 	return
		// }
		// tokenString := parts[1]

		claims, err := jwt.ParseAccessToken(authHeader)
		if err != nil {
			response.Fail(c, response.InvalidToken, "Token 无效或已过期")
			c.Abort()
			return
		}

		// 存储完整用户信息到context
		userInfo := &jwt.UserInfo{
			ID:       claims.UserID,
			Username: claims.Username,
			Role:     claims.Role,
		}
		c.Set("user_info", userInfo)
		c.Next()
	}
}

// RoleAuth 创建一个检查用户角色的中间件。
// 它必须在 Auth() 中间件之后使用。
// requiredRole: 访问该路由所需要的最低角色级别。
func RoleAuth(requiredRole int) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 从 context 中获取 Auth() 中间件设置的用户信息
		// c.Get() 返回的是一个 interface{} 和一个 bool 值
		val, exists := c.Get("user_info")
		if !exists {
			// 如果 user_info 不存在，说明 Auth() 中间件没有执行或失败了
			// 这是一种防御性编程，正常情况下不应该发生
			response.Fail(c, response.Unauthorized, "认证信息缺失，请先登录")
			c.Abort()
			return
		}
		// 2. 类型断言，将 interface{} 转换为我们需要的 *jwt.UserInfo 类型
		userInfo, ok := val.(*jwt.UserInfo)
		if !ok {
			// 如果类型断言失败，说明 context 中存储的值类型不正确
			// 这通常是开发中的错误，比如键名写错或存了其他类型的数据
			response.Fail(c, response.InternalError, "服务器内部错误：无法解析用户信息")
			c.Abort()
			return
		}
		// 3. 核心权限检查逻辑
		// 检查当前用户的角色级别是否大于等于所要求的角色级别
		// 这里的逻辑可以根据你的角色设计来定，通常数字越大级别越高
		if userInfo.Role < requiredRole {
			response.Fail(c, response.Forbidden, "权限不足，无法访问")
			c.Abort()
			return
		}
		// 4. 权限检查通过，放行请求
		c.Next()
	}
}
