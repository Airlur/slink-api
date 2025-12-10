package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Cors 处理跨域请求
func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		origin := c.Request.Header.Get("Origin")

		// 在这里可以配置允许的域名，目前为了方便开发，允许所有 Origin
		// 实际生产环境建议配置为具体的域名，例如 config.GlobalConfig.App.FrontendUrl
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin) 
			c.Header("Access-Control-Allow-Headers", "Content-Type, AccessToken, X-CSRF-Token, Authorization, Token, x-token")
			c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, DELETE, PATCH, PUT")
			c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type")
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		// 放行所有OPTIONS方法
		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
		}
		
		// 处理请求
		c.Next()
	}
}
