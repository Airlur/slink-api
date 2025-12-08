package errors

// Error 业务错误
type Error struct {
	Code    int    `json:"code"`    // 错误码
	Message string `json:"message"` // 错误信息
}

func (e *Error) Error() string {
	return e.Message
}

// New 创建业务错误
func New(code int, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}
