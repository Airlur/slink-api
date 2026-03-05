package response

import (
	"net/http"
	bizErrors "slink-api/internal/pkg/errors"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`    // 业务状态码
	Message string      `json:"message"` // 响应消息
	Data    interface{} `json:"data"`    // 响应数据
}


// ErrorInfo 存储了业务码对应的元信息
type ErrorInfo struct {
	HTTPStatus int    // 对应的HTTP状态码
	Message    string // 默认的错误提示
}

// errorMap 定义了业务码到错误信息的完整映射
var errorMap = map[int]ErrorInfo{
	// --- 通用码 ---
	Success: {http.StatusOK, "操作成功"},
	Failed:  {http.StatusInternalServerError, "操作失败"},

	// --- 用户相关 10xxx ---
	InvalidParam:     {http.StatusBadRequest, "请求参数错误"},
	Unauthorized:     {http.StatusUnauthorized, "用户未认证，请先登录"},
	InvalidToken:     {http.StatusUnauthorized, "令牌无效或已过期"},
	UserNotFound:     {http.StatusNotFound, "用户不存在"},
	PasswordError:    {http.StatusBadRequest, "用户名或密码错误"},
	UserAlreadyExist: {http.StatusConflict, "用户已存在"},
	UserStatusError:  {http.StatusForbidden, "用户状态异常"},
	Forbidden:        {http.StatusForbidden, "无权访问此资源"},
	LogoutFailed:     {http.StatusInternalServerError, "登出失败"},
	
	// --- 短链接相关 20xxx ---
	ShortlinkNotFound:             {http.StatusNotFound, "您查找的短链接不存在"},
	LinkNestingNotAllowed:         {http.StatusBadRequest, "不支持对服务自身域名进行缩短"},
	ShortcodeIsReserved:           {http.StatusBadRequest, "自定义短码是系统保留词，请更换"},
	ShortcodeHasRepetitiveChars:   {http.StatusBadRequest, "自定义短码包含过多连续重复字符"},
	InvalidExpiresInFormat:        {http.StatusBadRequest, "无效的有效期格式"},
	InvalidShortcodeChars:         {http.StatusBadRequest, "自定义短码只能包含字母和数字"},
	TooManyCustomAttempts:         {http.StatusBadRequest, "今日自定义尝试次数已达上限"},

	// --- 分享与标签 21xxx, 22xxx ---
	ShareNotFound: {http.StatusNotFound, "分享信息不存在"},
	TagNotFound:   {http.StatusNotFound, "标签不存在"},

	// --- 客户端/服务器错误 ---
	TooManyRequests: {http.StatusTooManyRequests, "请求过于频繁，请稍后再试"},
	InternalError:   {http.StatusInternalServerError, "服务器内部错误"},

    // TODO: 这里继续完善所有 code.go 中的业务码映射
}

// Ok 成功响应
func Ok(c *gin.Context, data interface{}, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: message,
		Data:    data,
	})
}


// Fail 失败响应 (已升级为智能版本)
func Fail(c *gin.Context, code int, message string) {
	// 从map中查找错误信息
	errorInfo, ok := errorMap[code]
	if !ok {
		// 如果map里没有定义，默认按内部错误处理
		errorInfo = errorMap[InternalError]
	}

	// 如果传入的message为空，则使用map中定义的默认message
	if message == "" {
		message = errorInfo.Message
	}

	c.JSON(errorInfo.HTTPStatus, Response{
		Code:    code,
		Message: message,
		Data:    nil,
	})
}

// Fail 失败响应(使用默认HTTP状态码200) 【最初版本】
// func Fail(c *gin.Context, code int, message string) {
// 	c.JSON(http.StatusOK, Response{
// 		Code:    code,
// 		Message: message,
// 		Data:    nil,
// 	})
// }

// FailWithStatus 失败响应(指定HTTP状态码)
func FailWithStatus(c *gin.Context, httpStatus, code int, message string) {
	c.JSON(httpStatus, Response{
		Code:    code,
		Message: message,
		Data:    nil,
	})
}

func Error(c *gin.Context, err error) {
	if e, ok := err.(*bizErrors.Error); ok {
		Fail(c, e.Code, e.Message)
		return
	}
	Fail(c, http.StatusInternalServerError, err.Error())
}
