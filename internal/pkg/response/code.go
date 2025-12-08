package response

// 业务错误码
const (
	// 通用错误码
	Success = 0  // 成功
	Failed  = -1 // 未知错误

	// 工具类相关 0xxxx
	GenerateTokenFailed = 00001 // 生成token失败

	// 用户相关 1xxxx
	InvalidParam      = 10001 // 参数错误
	Unauthorized      = 10002 // 未登录
	InvalidToken      = 10003 // 无效的token
	UserNotFound      = 10004 // 用户不存在
	PasswordError     = 10005 // 密码错误
	UserAlreadyExist  = 10006 // 用户已存在
	UserStatusError   = 10007 // 用户状态错误
	Forbidden         = 10008 // 无权限
	LogoutFailed      = 10009 // 登出失败
	DuplicateEntry	  = 10010 // 数据重复

	// 短链接相关 20xxx
	ShortlinkNotFound 			= 20004 // 短链接不存在
	LinkNestingNotAllowed     	= 20005 // 不支持对本服务域名进行二次缩短
	ShortcodeIsReserved       	= 20006 // 自定义短码是系统保留词
	ShortcodeHasRepetitiveChars = 20007 // 自定义短码包含过多连续重复字符
	InvalidExpiresInFormat    	= 20008 // 无效的有效期格式
	InvalidShortcodeChars     	= 20009 // 自定义短码包含非法字符
	TooManyCustomAttempts 		= 20010 // 自定义短码尝试次数过多

	// 短链接-分享 相关 21xxx
	ShareNotFound				= 21004 // 短链接分享内容不存在

	// 短链接-标签 相关 22xxx
	TagNotFound					= 22004 // 短链接标签不存在

	// 客户端错误状态码 4xx
	TooManyRequests	  = 429 // 请求过于频繁，超出了“频次限制”
	

	// 服务器内部错误相关 5xx
	InternalError	  = 500 // 服务器内部错误

	// 其他业务错误码...
)
