一、项目概述
1.1 核心定位
一款轻量级短链接服务，支持长链接转短链接（自动 / 自定义短码）、短链接跳转、多维度访问统计及分级用户管理，兼顾易用性、安全性与高性能，适配个人及小型团队使用，同时作为 Go 语言后端开发实践项目。
1.2 核心价值
● 链路优化：将冗长链接缩短为 6-8 位 Base62 短码（区分大小写），提升传播效率与美观度；
● 数据可控：区分普通用户 / 管理员维度，提供基础统计、渠道分析、地域分布等多维度数据查询能力；
● 安全可靠：覆盖 URL 格式校验、恶意内容拦截、多级限流防护、HTTPS 强制加密，规避滥用与数据泄露风险；
● 功能闭环：支持账号管理（注册 / 登录 / 找回密码 / 注销）、短链生命周期管理（失效预警 / 有效期延长），满足全场景使用需求；
● 易用扩展：提供 Web 管理后台与在线 API 文档，支持二次开发对接，降低非技术用户使用门槛。
二、功能需求与模块划分
1. 用户与权限模块
1.1 核心功能
● 用户注册 / 登录
  ○ 注册：支持用户名 / 邮箱 + 密码注册，密码采用 bcrypt 加密存储，注册前需同意《隐私政策》；
  ○ 登录：支持邮箱 / 用户名 + 密码登录，返回含用户 ID、角色的 JWT Token（TTL=7 天），支持 Token 刷新（剩余有效期≤1 天时可获取新 Token）；
  ○ 登录保护：连续 5 次密码错误锁定账号 1 小时，支持邮箱验证码解锁。
● 角色划分与权限
角色	核心权限	限制条件
普通用户	管理个人短链接（查看 / 失效 / 延长有效期）、查看个人短链统计、设置短链标签、导出统计数据	每日生成短链≤100 个
管理员	管理所有用户（封禁 / 解封 / 重置密码）、管理全局短链（批量失效 / 清理）、查看全局统计数据	无生成数量限制，拥有所有接口访问权限
● 账号管理
  ○ 密码找回：通过 “邮箱验证码” 流程重置密码（验证码有效期 15 分钟），重置后自动登出所有已登录设备；
  ○ 账号注销：需 “当前密码 + 邮箱验证码” 双重验证，注销后保留 30 天数据恢复期，到期后彻底删除用户数据（含短链、日志）；
  ○ 个人信息查询：支持通过/api/v1/user/info接口获取用户名、邮箱、最近登录时间等信息。
2. 短链接核心模块
2.1 核心功能
● 长链接转短链接
  ○ 自动生成短码：默认生成 6-8 位 Base62 短码（0-9、a-z、A-Z，区分大小写），基于数据库自增 ID 编码，生成后校验 Redis + 数据库确保唯一性（极端场景重试 3 次）；
  ○ 自定义短码：支持用户传入 6-8 位 Base62 字符短码，需校验唯一性（Redis + 数据库双重确认）、过滤预留词（如 “admin”“api”“login”）及连续 6 位相同字符（如 “111111”）；
  ○ 防嵌套：解析长链接域名与路径，若匹配本服务短链接格式（域名一致 + 路径符合短码规则），禁止再次转链；
  ○ 短链接有效期：支持小时 / 天 / 周 / 月 / 年 / 永久 6 种有效期选择，存储时记录expire_at字段（永久则为 NULL）。
● 短链接跳转
  ○ 跳转逻辑：解析短码后校验有效性（未过期、未失效），有效则 302 重定向至原始链接，无效返回 404；
  ○ 安全加密：所有跳转请求强制使用 HTTPS，集成 Let's Encrypt SSL 证书自动续期。
● 短链接管理
  ○ 普通用户：查看个人短链列表（分页 + 按标签筛选 + 点击量排序）、手动标记短链失效、延长有效期（支持 1/7/30 天）、设置短链标签（如 “活动推广”）；
  ○ 管理员：查看全局短链列表、批量标记失效 / 删除、清理过期短链（按日期筛选）；
  ○ 失效预警：非永久短链到期前 3 天，向用户邮箱发送预警邮件，含 “一键延长有效期” 入口。
● 短链接分享
  ○ 支持自定义分享卡片信息（share_title≤50 字、share_desc≤100 字、share_image为 URL），适配微信 / 微博等平台链接预览规则；
  ○ 原始链接脱敏：管理页与 API 返回中隐藏原始链接中间部分（如https://xxx.com/.../product?id=123），仅管理员可查看完整链接。
3. 访问统计模块
3.1 核心功能
● 日志记录：记录短链访问信息，包括短码、访问 IP、设备 User-Agent、访问时间、用户 ID、地域（省份 / 城市）、来源渠道、操作系统版本、浏览器类型。
● 统计查询
  ○ 基础统计：短链接总点击量、最近 10 次访问记录；
  ○ 多维度统计：近 7/30 天点击趋势、设备分布（PC / 手机 + 操作系统 + 浏览器）、地域分布（TOP10 省份 / 城市）、来源渠道分布（直接访问 / 微信 / 微博等）；
  ○ 全局统计（仅管理员）：平台总短链数、总点击量、活跃用户数（近 30 天有操作的用户）、TOP5 热门短链。
● 数据导出：普通用户可导出个人短链统计数据（支持自定义时间范围），CSV 文件含点击量、地域、渠道、操作系统等字段；管理员可导出全局统计数据。
4. 安全防护模块
4.1 核心功能
● URL 安全处理
  ○ 格式校验：正则校验^(https?://)?([a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}(:\d{1,5})?(/.*)?$，无协议自动补 HTTPS，最长支持 2048 字符；
  ○ 恶意检测：创建短链前调用阿里云内容安全 API，拦截钓鱼 / 病毒 / 敏感内容链接，记录黑名单（1 小时内不再重复校验）；
  ○ 特殊字符处理：对中文、空格等执行 URLEncode 编码，存储编码后的值。
● 多级限流防护
限流场景	规则
限流中间件
中间件名称	维度	核心作用	实现原理	适用范围	建议配置示例
RateLimitLinkCreate	用户 / IP	限制短链接创建频率，按日重置	固定窗口计数（INCR + EXPIRENX）	短链接创建接口	- 登录用户：50 次 / 天
- 游客（IP）：20 次 / 天
RateLimitLinkAccess	IP	限制短链接跳转频率，允许合理突发	令牌桶算法（redis_rate库）	短链接跳转接口	- 速率：50 次 / 秒
- 突发容量：100 次
IPBlockMiddleware	IP	识别并拦截短时间恶意攻击	滑动窗口（ZSet 存储时间戳）	全局接口防护	- 1 分钟内超过 100 次请求则临时拉黑
- 拉黑时长：10 分钟
RateLimitAccount	账号（用户 ID）	限制单个用户的操作频率	固定窗口计数（复用IncrWithExpiration）	登录、信息修改、短链接管理等需登录接口	- 登录接口：10 次 / 分钟
- 短链接创建：20 次 / 天
- 信息修改：5 次 / 分钟
RateLimitDevice	设备（设备指纹）	限制单个设备的操作频率	固定窗口计数（INCR + EXPIRENX）	注册、匿名访问、验证码发送等未登录场景	- 注册接口：3 次 / 天
- 验证码发送：5 次 / 小时
- 匿名操作：20 次 / 小时
RateLimitGlobal	系统全局	限制服务总 QPS，保护整体稳定性	令牌桶算法（单一全局 key）	所有接口（部署在路由最上层）	- 总 QPS：10000 次 / 秒（根据服务器性能调整）
RateLimitSensitive	敏感操作	对高风险操作额外增强限流	固定窗口计数（更严格阈值）	密码重置、权限变更、支付相关（若有）	- 密码重置：3 次 / 分钟
- 权限变更：2 次 / 分钟
限流的业务场景
短链生成限流	普通用户每日≤100 个，管理员无限制
短链访问限流	热点短码（日点击≥1000 次）每秒≤100 次，非热点短码每秒≤50 次
自定义短码限流	单用户每日自定义短码尝试≤50 次，避免恶意占用
异常 IP 防护	首次访问 IP 前 5 分钟请求≤50 次，1 分钟内≥100 次请求临时限制（10 分钟禁止操作）

用户模块
具体接口 / 功能	风险点	限流方案（维度 + 阈值）	实现方式
用户名 / 邮箱唯一性校验	恶意探测系统内用户信息	- IP 维度：10 次 / 秒
- 设备维度：30 次 / 分钟	业务代码中调用IncrWithExpiration工具函数
用户注册	批量注册垃圾账号	- IP 维度：5 次 / 小时
- 设备维度：3 次 / 天
- 全局维度：100 次 / 分钟	组合使用RateLimitIP和RateLimitDevice中间件
密码重置（含验证码）	刷验证码、暴力尝试	- 账号维度：3 次 / 小时
- IP 维度：10 次 / 小时	业务代码中结合账号限流逻辑
短链接模块
具体接口 / 功能	风险点	限流方案（维度 + 阈值）	实现方式
短链接删除 / 修改	高频操作导致数据不一致	- 用户维度：20 次 / 天
- IP 维度：5 次 / 分钟	业务代码中调用限流工具函数
短链接统计查询	高频查询消耗数据库资源	- 用户维度：60 次 / 分钟
- IP 维度：30 次 / 分钟	业务代码中调用限流工具函数
公共功能模块
具体接口 / 功能	风险点	限流方案（维度 + 阈值）	实现方式
图片上传（如封面）	消耗带宽 / 存储资源	- 用户维度：10 张 / 天
- IP 维度：5 张 / 分钟	组合使用RateLimitAccount和RateLimitIP中间件
搜索功能（如历史短链）	高频查询消耗 CPU	- 用户维度：30 次 / 分钟
- IP 维度：60 次 / 分钟	业务代码中调用限流工具函数

● 数据合规：提供/api/v1/policy接口与 Web 端《隐私政策》页面，明确数据收集范围（IP、设备信息、访问日志）、使用目的（统计 / 安全防护）、保留期限（日志保留 90 天）。
5. 系统配套模块
5.1 核心功能
● Web 管理后台（基于 Vue3+Element Plus）
  ○ 普通用户端：短链列表管理、统计图表查看（趋势图 / 饼图）、标签管理、数据导出；
  ○ 管理员端：全局数据看板、用户管理（封禁 / 解封 / 重置密码）、全局短链管理、系统监控指标查看。
● 在线 API 文档：基于 Go-Swagger 生成，访问路径/api/v1/docs，包含接口参数说明、请求示例、响应码解释、错误码列表，支持在线调试。
● 系统监控：集成 Prometheus+Grafana，监控接口响应时间、缓存命中率、数据库连接数、短链生成量等指标，异常时通过管理员邮箱告警。
三、核心表结构与索引设计
1. 用户表（users）
字段名	数据类型	约束条件	说明
id	BIGINT	PRIMARY KEY, AUTO_INCREMENT	用户唯一标识
username	VARCHAR(50)	NOT NULL, UNIQUE	登录用户名
email	VARCHAR(100)	NOT NULL, UNIQUE	登录邮箱
password_hash	VARCHAR(255)	NOT NULL	bcrypt 加密后的密码
role	VARCHAR(20)	NOT NULL, DEFAULT 'user'	角色（user/admin）
status	TINYINT	NOT NULL, DEFAULT 1	状态（1 = 正常，0 = 封禁）
status_reason	VARCHAR(100)	NULL	封禁原因（仅状态为 0 时有效）
last_login_at	DATETIME	NULL	最近登录时间
created_at	DATETIME	DEFAULT CURRENT_TIMESTAMP	创建时间
updated_at	DATETIME	DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP	更新时间
索引	——	——	PRIMARY KEY(id)；UNIQUE(username, email)
2. 短链接表（short_links）
字段名	数据类型	约束条件	说明
id	BIGINT	PRIMARY KEY, AUTO_INCREMENT	短链接唯一标识
short_code	VARCHAR(20)	NOT NULL, UNIQUE	短码（Base62，区分大小写）
original_url	TEXT	NOT NULL	原始长链接（URLEncode 编码）
user_id	BIGINT	NOT NULL	关联用户 ID（管理员为 0）
expire_at	DATETIME	NULL	过期时间（NULL = 永久）
last_warn_at	DATETIME	NULL	最近一次失效预警发送时间
status	TINYINT	NOT NULL, DEFAULT 1	状态（1 = 有效，0 = 失效）
click_count	BIGINT	NOT NULL, DEFAULT 0	点击量统计
is_hot	TINYINT	DEFAULT 0	是否为热点短码（1 = 是，日访问≥1000）
is_custom	TINYINT	DEFAULT 0	是否为自定义短码（1 = 是）
created_at	DATETIME	DEFAULT CURRENT_TIMESTAMP	创建时间
updated_at	DATETIME	DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP	更新时间
索引	——	——	PRIMARY KEY(id)；UNIQUE(short_code)；INDEX(user_id, status)；INDEX(expire_at)
3. 访问日志表（access_logs）
字段名	数据类型	约束条件	说明
id	BIGINT	PRIMARY KEY, AUTO_INCREMENT	日志唯一标识
short_code	VARCHAR(20)	NOT NULL	关联短码
user_id	BIGINT	NOT NULL	访问者用户 ID（非登录用户为 0）
ip	VARCHAR(45)	NOT NULL	访问 IP 地址
user_agent	VARCHAR(512)	NULL	设备 User-Agent
device_type	VARCHAR(20)	NULL	设备类型（PC / 手机 / 其他）
os_version	VARCHAR(50)	NULL	操作系统版本
browser	VARCHAR(50)	NULL	浏览器类型
region	VARCHAR(50)	NULL	访问地域（省份 / 城市）
channel	VARCHAR(50)	NULL	来源渠道（直接访问 / 微信 / 微博等）
accessed_at	DATETIME	DEFAULT CURRENT_TIMESTAMP	访问时间
索引	——	——	PRIMARY KEY(id)；INDEX(short_code, accessed_at)；INDEX(accessed_at)；INDEX(region, channel)
4. 短链标签表（shortlink_tags）
字段名	数据类型	约束条件	说明
id	BIGINT	PRIMARY KEY, AUTO_INCREMENT	标签唯一标识
short_code	VARCHAR(20)	NOT NULL	关联短码
user_id	BIGINT	NOT NULL	关联用户 ID
tag_name	VARCHAR(30)	NOT NULL	标签名称（如 “活动推广”）
created_at	DATETIME	DEFAULT CURRENT_TIMESTAMP	创建时间
索引	——	——	INDEX(user_id, short_code)；INDEX(tag_name)
5. 短链分享信息表（shortlink_shares）
字段名	数据类型	约束条件	说明
id	BIGINT	PRIMARY KEY, AUTO_INCREMENT	分享信息唯一标识
short_code	VARCHAR(20)	NOT NULL, UNIQUE	关联短码
share_title	VARCHAR(50)	NULL	分享标题
share_desc	VARCHAR(100)	NULL	分享描述
share_image	VARCHAR(255)	NULL	分享封面图 URL
created_at	DATETIME	DEFAULT CURRENT_TIMESTAMP	创建时间
updated_at	DATETIME	DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP	更新时间
索引	——	——	PRIMARY KEY(id)；UNIQUE(short_code)
四、核心难点与解决方案
1. 短码生成与唯一性保障
● 难点：自定义短码区分大小写的唯一性校验、自动短码抗猜测与冲突；
● 解决方案：
  ○ 自定义短码：先查 Redis（key=short_code:custom:{短码}），再查数据库，双重确认未占用，冲突时返回推荐自动短码；
  ○ 自动短码：基于数据库自增 ID 生成 Base62 编码，不足 6 位补 1-2 个随机字符，生成后校验 Redis + 数据库，重试 3 次规避冲突。
2. 性能优化（高并发与缓存）
● 难点：高频跳转导致数据库压力、缓存穿透 / 击穿 / 雪崩；
● 解决方案：
  ○ 多级缓存：Redis 缓存（TTL=24h±1h 随机偏移）→本地内存缓存（热点短码）→数据库；
  ○ 异步处理：goroutine + 带缓冲 Channel（容量 1000）异步更新点击量与日志，批量写入数据库；
  ○ 缓存问题防护：无效短码缓存（Redis value=null，TTL=5min）、热点短码永不过期（后台 10 分钟更新一次）、Redis 主从架构（1 主 2 从）。
3. 数据量与合规
● 难点：访问日志激增导致数据库性能下降、数据合规与隐私保护；
● 解决方案：
  ○ 日志分表：按月份分表（如 access_logs_202405），单表数据量控制在百万级；
  ○ 定时清理：每天凌晨 2 点删除 90 天前日志、30 天前失效短链，1 年前日志归档为 CSV；
  ○ 隐私保护：原始链接脱敏展示、仅存储必要用户数据、日志保留期限明确（90 天）。
五、核心 API 接口设计（RESTful 风格）
1. 用户与权限模块接口
接口路径	方法	功能	鉴权	请求体示例	响应示例
/api/v1/user/register	POST	用户注册	无需	{"username":"testuser","email":"test@x.com","password":"Abc123456","policy_agreed":true}	{"code":200,"data":{"user_id":1001,"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."},"msg":"注册成功"}
/api/v1/user/login	POST	用户登录	无需	{"email":"test@x.com","password":"Abc123456"}	{"code":200,"data":{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...","role":"user"},"msg":"登录成功"}
/api/v1/user/token/refresh	POST	刷新 JWT Token	需 JWT	{"old_token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."}	{"code":200,"data":{"new_token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."},"msg":"Token 刷新成功"}
/api/v1/user/password/forgot	POST	发起密码找回	无需	{"email":"test@x.com"}	{"code":200,"msg":"验证码已发送至邮箱，有效期15分钟"}
/api/v1/user/password/reset	POST	重置密码	无需	{"email":"test@x.com","code":"123456","new_password":"NewAbc123"}	{"code":200,"msg":"密码重置成功，请重新登录"}
/api/v1/user/logout	POST	用户登出（销毁 Token）	需 JWT	{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."}	{"code":200,"msg":"登出成功，Token 已失效"}
/api/v1/user/info	GET	获取个人信息	需 JWT	——	{"code":200,"data":{"username":"testuser","email":"test@x.com","last_login_at":"2024-11-01 14:30:00"}}
/api/v1/user/cancel	POST	账号注销	需 JWT	{"password":"Abc123456","code":"654321"}	{"code":200,"msg":"账号注销申请成功，30天内可申请恢复"}
/api/v1/admin/user/ban	POST	封禁用户	需管理员 JWT	{"user_id":1001,"reason":"恶意生成违规链接","ban_days":30}	{"code":200,"data":{"user_id":1001,"ban_until":"2024-12-01 14:30:00"},"msg":"用户封禁成功"}
/api/v1/admin/user/unban	POST	解封用户	需管理员 JWT	{"user_id":1001}	{"code":200,"msg":"用户解封成功"}
2. 短链接核心模块接口
接口路径	方法	功能	鉴权	请求体示例	响应示例
/api/v1/shortlink/create	POST	生成短链接	需 JWT	{"original_url":"https://www.example.com/long-url-path","expire_at":"2024-12-31 23:59:59","custom_code":"Test123"}	{"code":200,"data":{"short_url":"https://s.x.com/Test123","short_code":"Test123","is_custom":1},"msg":"短链接生成成功"}
/api/v1/shortlink/list	GET	查询短链接列表	需 JWT	?page=1&size=10&status=1&tag_name=活动推广	{"code":200,"data":{"total":25,"list":[{"short_code":"Test123","original_url":"https://www.example.com/...","click_count":120,"expire_at":"2024-12-31 23:59:59"}],"page":1,"size":10}}
/:short_code	GET	短链接跳转	无需	——	302 重定向至原始 URL（无效短码返回 {"code":404,"msg":"短链接不存在或已失效"}
）
/api/v1/shortlink/disable	POST	标记短链接失效	需 JWT	{"short_code":"Test123"}	{"code":200,"msg":"短链接已标记失效"}
/api/v1/shortlink/extend	POST	延长短链有效期	需 JWT	{"short_code":"Test123","extend_days":7}	{"code":200,"data":{"new_expire_at":"2025-01-07 23:59:59"},"msg":"有效期延长成功"}
/api/v1/shortlink/info	GET	查询短链接详情	需 JWT	?short_code=Test123	{"code":200,"data":{"short_code":"Test123","original_url":"https://www.example.com/...","click_count":120,"expire_at":"2024-12-31 23:59:59","status":1}}
/api/v1/shortlink/share/set	POST	设置短链分享信息	需 JWT	{"short_code":"Test123","share_title":"产品推广链接","share_desc":"点击查看最新产品","share_image":"https://www.example.com/img.jpg"}	{"code":200,"data":{"share_info":{"title":"产品推广链接","desc":"点击查看最新产品","image":"https://www.example.com/img.jpg"}},"msg":"分享信息设置成功"}
/api/v1/admin/shortlink/list	GET	查询全局短链列表	需管理员 JWT	?page=1&size=20&status=1	{"code":200,"data":{"total":150,"list":[{"short_code":"Test123","user_id":1001,"click_count":120,"expire_at":"2024-12-31 23:59:59"}],"page":1,"size":20}}
/api/v1/admin/shortlink/clean	POST	批量清理过期短链	需管理员 JWT	{"before_date":"2024-10-01 00:00:00"}	{"code":200,"data":{"clean_count":32},"msg":"过期短链清理成功"}
3. 访问统计模块接口
接口路径	方法	功能	鉴权	请求体示例	响应示例
/api/v1/stat/basic	GET	获取短链基础统计	需 JWT	?short_code=Test123	{"code":200,"data":{"total_click":120,"last_10_logs":[{"ip":"123.45.67.89","accessed_at":"2024-11-01 14:30:00"}]}}
/api/v1/stat/trend	GET	获取点击趋势统计	需 JWT	?short_code=Test123&days=7	{"code":200,"data":{"trend":[{"date":"2024-10-26","click_count":15},{"date":"2024-10-27","click_count":22}]}}
/api/v1/stat/region	GET	获取地域分布统计	需 JWT	?short_code=Test123&start_date=2024-11-01&end_date=2024-11-30	{"code":200,"data":{"regions":[{"name":"广东省","count":35},{"name":"北京市","count":28}],"total":120}}
/api/v1/stat/channel	GET	获取来源渠道统计	需 JWT	?short_code=Test123	{"code":200,"data":{"channels":[{"name":"微信","count":60},{"name":"直接访问","count":35},{"name":"微博","count":25}]}}
/api/v1/stat/device	GET	获取设备详情统计	需 JWT	?short_code=Test123	{"code":200,"data":{"devices":[{"os":"iOS 17.1","browser":"Safari 16","count":45},{"os":"Android 14","browser":"Chrome 120","count":55}]}}
/api/v1/stat/export	POST	导出统计数据（CSV）	需 JWT	{"short_code":"Test123","start_date":"2024-11-01","end_date":"2024-11-30"}	{"code":200,"data":{"download_url":"https://s.x.com/export/stat_1001_202411.csv"},"msg":"数据导出成功，链接有效期24小时"}
/api/v1/admin/stat/global	GET	获取全局统计	需管理员 JWT	?days=30	{"code":200,"data":{"total_shortlinks":150,"total_clicks":5200,"active_users":85,"top_5_links":[{"short_code":"Hot123","click_count":1200}]}}
4. 系统配套模块接口
接口路径	方法	功能	鉴权	请求体示例	响应示例
/api/v1/policy	GET	获取隐私政策	无需	——	{"code":200,"data":{"content":"隐私政策内容：1. 数据收集范围...","update_time":"2024-10-01 00:00:00"}}
/api/v1/docs	GET	访问在线 API 文档	无需	——	返回 Swagger 在线文档页面
/api/v1/admin/monitor	GET	获取系统监控指标	需管理员 JWT	——	{"code":200,"data":{"api_response_time":50,"cache_hit_rate":98,"db_connections":20,"total_requests":12000}}
六、核心难点与解决方案（补充完善）
1. 短码生成与唯一性保障（补充）
● 难点：自定义短码区分大小写导致用户易混淆（如Test123与test123），且手动输入易出错；
● 解决方案：
  a. 前端输入时实时提示 “短码区分大小写”，并提供预览（如输入test123时显示 “与已存在的Test123不同，可使用”）；
  b. 自定义短码提交前，前端自动过滤全角字符（如 “１２３” 转为 “123”），避免编码不一致导致的冲突。
2. 短链失效预警与通知
● 难点：用户可能未查看邮箱，导致错过短链失效预警；
● 解决方案：
  a. 登录用户在 Web 管理后台 “短链列表” 中，对即将过期的短链标注 “[即将过期]” 标签（红色提醒）；
  b. 预警邮件中附加 “一键延长有效期” 直接链接，点击即可跳转至延长页面（无需二次登录，通过临时 Token 验证）。
3. 第三方 API 依赖风险（如 IP 定位、内容安全）
● 难点：第三方 API 故障或收费升级可能导致地域统计、恶意检测功能失效；
● 解决方案：
  a. 设计降级策略：第三方 API 不可用时，地域统计默认显示 “未知”，恶意检测仅执行基础关键词过滤；
  b. 预留多 API 厂商配置（如同时支持高德、百度 IP 定位），可通过配置文件快速切换备用 API。
七、项目开发优先级清单
第一阶段：核心功能开发（优先级最高，确保服务可用）
目标
实现短链接 “生成 - 跳转 - 基础管理” 核心链路，满足用户最基本的短链使用需求。
开发内容
1. 用户与权限模块核心功能
  ○ 用户注册 / 登录（含 JWT 生成与校验）；
  ○ 普通用户 / 管理员角色权限划分（基础数据隔离）；
  ○ 登录失败锁定与邮箱解锁。
2. 短链接核心模块核心功能
  ○ 自动生成短码（Base62，区分大小写）与长链转短链；
  ○ 短码唯一性校验（Redis + 数据库双重确认）；
  ○ 短链接跳转（302 重定向）与有效性校验（过期 / 失效判断）；
  ○ 普通用户查看 / 标记个人短链失效，管理员查看全局短链。
3. 安全防护基础功能
  ○ URL 格式校验（补全 HTTPS、过滤超长链接）；
  ○ 单 IP 限流（短链生成 / 访问）；
  ○ HTTPS 强制启用（配置 SSL 证书）。
4. 数据存储与基础性能
  ○ 核心表结构创建（users、short_links、access_logs）；
  ○ Redis 基础缓存（短链信息缓存，TTL=24h）；
  ○ 点击量异步更新（goroutine+Channel 批量写入）。
交付物
● 可运行的后端服务（支持短链生成、跳转、基础管理）；
● 核心 API 接口（注册 / 登录、短链创建 / 跳转 / 失效）；
● 基础数据库脚本与 Redis 配置。
第二阶段：重要功能开发（优先级中，完善功能闭环）
目标
补充账号管理、多维度统计、短链生命周期管理，解决用户 “用得爽” 的问题。
开发内容
1. 用户与权限模块重要功能
  ○ 密码找回（邮箱验证码流程）；
  ○ 账号注销（含 30 天数据恢复期）；
  ○ JWT Token 刷新机制。
2. 短链接核心模块重要功能
  ○ 自定义短码（含预留词过滤、连续字符校验）；
  ○ 短链接有效期设置与延长；
  ○ 原始链接脱敏展示（管理页与 API 返回）。
3. 访问统计模块核心功能
  ○ 基础统计（短链总点击量、最近 10 次访问日志）；
  ○ 点击趋势统计（近 7 天）与设备类型统计（PC / 手机）；
  ○ 普通用户统计数据导出（CSV，基础字段）。
4. 安全防护重要功能
  ○ 恶意 URL 检测（调用第三方内容安全 API）；
  ○ 短链防嵌套（识别自身短链接，禁止二次转链）；
  ○ 敏感词过滤（基础违规关键词库）。
5. 系统配套基础功能
  ○ 在线 API 文档（Swagger 生成，含核心接口说明）；
  ○ 数据库分表（access_logs 按月份分表）。
交付物
● 完整的账号管理功能（注册 - 登录 - 找回密码 - 注销）；
● 多维度统计报表（基础 + 趋势 + 设备）；
● 在线 API 文档（可调试）；
● 分表存储脚本与定时清理任务（删除 90 天前日志）。
第三阶段：优化功能开发（优先级低，提升体验与扩展性）
目标
通过个性化功能、可视化工具与监控能力，提升用户使用体验，降低运维与扩展成本，满足多样化场景需求。
开发内容
1. 用户与权限模块优化功能
  ○ 个人信息展示（最近登录时间、生成短链总数）；
  ○ 账号注销后的数据恢复流程（30 天内支持通过邮箱申请恢复）。
2. 短链接核心模块优化功能
  ○ 短链标签管理（普通用户添加 / 删除标签，按标签筛选短链）；
  ○ 短链分享信息设置（自定义标题、描述、封面图，适配平台预览规则）；
  ○ 短链失效预警（到期前 3 天邮箱通知 + Web 后台红色标签提醒，含一键延长链接）。
3. 访问统计模块优化功能
  ○ 地域分布统计（调用 IP 定位 API，展示 TOP10 省份 / 城市点击占比）；
  ○ 来源渠道统计（解析 Referer，区分微信 / 微博 / 直接访问等渠道）；
  ○ 设备详情统计（补充操作系统版本、浏览器类型数据）；
  ○ 统计数据导出优化（支持自定义时间范围，CSV 新增地域 / 渠道字段）。
4. 安全防护优化功能
  ○ 第三方 API 降级策略实现（IP 定位 / 内容安全 API 故障时默认降级）；
  ○ 多 API 厂商配置（支持切换 IP 定位 / 内容安全的备用 API）；
  ○ 自定义短码前端优化（实时提示区分大小写、过滤全角字符）。
5. 系统配套优化功能
  ○ Web 管理后台开发（普通用户端：短链管理 / 统计图表；管理员端：全局数据 / 用户管理）；
  ○ 系统监控集成（Prometheus+Grafana，监控接口响应时间、缓存命中率等指标）；
  ○ 隐私政策页面开发（含接口/api/v1/policy与 Web 端展示）。
交付物
● 功能完整的 Web 管理后台（前后端联调完成）；
● 多维度统计报表（含地域 / 渠道 / 设备详情）；
● 系统监控看板与告警配置；
● 隐私政策文档与展示页面。
第四阶段：迭代与扩展功能（优先级最低，按需迭代）
目标
基于用户反馈优化现有功能，扩展服务适用场景，提升服务稳定性与可扩展性。
开发内容
1. 功能迭代优化
  ○ 根据用户反馈调整 Web 管理后台交互（如简化短链创建表单）；
  ○ 优化短码生成算法（如减少相似字符 “0/O”“1/l” 的出现概率）；
  ○ 扩展短链有效期选项（支持自定义具体时间，如 “2024-12-31 18:00:00”）。
2. 扩展功能
  ○ 短链访问密码保护（用户可设置密码，访问时需输入密码才能跳转）；
  ○ 批量短链生成（支持上传 CSV 批量转换长链为短链，适配团队运营场景）；
  ○ 开放平台 API（支持第三方应用通过 Access Key 调用短链生成接口，适配二次开发）。
3. 性能与稳定性优化
  ○ 优化缓存策略（如基于访问频率动态调整热点短码缓存时间）；
  ○ 数据库读写分离（主库写入，从库负责统计查询，降低主库压力）；
  ○ 日志归档自动化（1 年前日志自动归档为 CSV 并迁移至低成本存储，如对象存储）。
交付物
● 迭代优化后的功能版本（含用户反馈修复）；
● 批量短链生成 / 访问密码保护等扩展功能；
● 数据库读写分离配置与性能测试报告；
● 开放平台 API 文档与接入示例。
八、项目交付标准
1. 功能验收标准
模块	验收要点
用户与权限模块	注册 / 登录成功率 100%，JWT Token 有效期准确，密码找回流程 15 分钟内收到验证码
短链接核心模块	短链生成成功率≥99.9%，跳转响应时间≤100ms，自定义短码区分大小写且无冲突
访问统计模块	统计数据延迟≤5 分钟，导出 CSV 字段完整，地域 / 渠道识别准确率≥90%
安全防护模块	HTTPS 强制生效，限流规则触发时返回正确 429 状态码，恶意 URL 拦截率≥95%
系统配套模块	Web 管理后台页面加载时间≤2s，API 文档接口示例可正常调试，监控告警响应及时
2. 性能验收标准
● 支持单节点每秒 1000 + 短链跳转请求，CPU 使用率≤70%，内存占用≤512MB；
● Redis 缓存命中率≥95%，数据库查询响应时间≤50ms（非统计类查询）；
● 支持每日 10 万 + 短链生成，访问日志存储量≥100 万条 / 月（分表后无性能瓶颈）。
3. 文档交付标准
● 包含项目部署文档（环境依赖、配置步骤、启动命令）；
● 包含 API 接口文档（Swagger 在线版 + 离线 Markdown 版）；
● 包含数据库设计文档（表结构、索引、分表策略）；
● 包含运维手册（监控指标说明、常见问题排查、数据备份恢复步骤）。
九、风险与应对措施
风险类型	具体风险	应对措施
技术风险	Redis 集群故障导致缓存失效，数据库压力激增	部署 Redis 主从架构（1 主 2 从），开启持久化（RDB+AOF），配置缓存降级策略
第三方依赖风险	第三方 IP 定位 / 内容安全 API 收费上涨或停止服务	预留多厂商 API 配置，开发降级功能（如默认显示 “未知地域”“基础关键词过滤”）
业务风险	短链被用于传播违规内容，导致平台风险	加强恶意 URL 检测，新增用户举报功能，管理员可实时封禁违规短链
运维风险	数据误删除或服务器故障导致数据丢失	数据库每日自动备份（全量 + 增量），备份文件保留 30 天，支持跨节点数据恢复


// 自定义 HTTP 头（X-Real-IP）来模拟客户端 IP 地址，告知服务器 “真实的客户端 IP”（在本机运行或有代理的场景下）。
curl -H "X-Real-IP: 139.162.37.41" http://localhost:8080/f24SA3NC
curl -H "X-Real-IP: 211.148.214.255" http://localhost:8080/f24SA3NC
curl -H "X-Real-IP: 201.188.222.255" http://localhost:8080/f24SA3NC

十、✅ 项目最终状态总览 (Final Project Review)
第一部分：核心基础架构 (Core Infrastructure)
配置管理 (config): 建立了类型安全、环境隔离的配置体系。

日志系统 (logger): 集成了高性能的zap日志库，实现了结构化日志。

数据库 (gorm): 统一了Repository层的数据访问模式，实现了健壮的错误处理。

缓存 (redis): 封装了常用的Redis操作，并为核心缓存提供了GetAndDel等原子操作能力。

响应与错误处理 (response, errors): 创建了一套完整的、职责分明的错误处理系统，能够根据业务码智能映射HTTP状态码。

代码生成模板: 我们将所有核心层（model, dto, repo, service, api）的模板都进行了现代化升级，能够生成高质量的模块骨架。

第二部分：用户与认证模块 (User & Auth Module)
认证体系: 搭建了基于**双Token（Access + Refresh）**的、有状态的（Stateful JWT）安全认证系统，实现了性能与安全的平衡。

核心流程: 实现了注册、登录、登出、刷新Token的完整会话管理流程。

密码安全:

实现了用户修改密码（已登录状态）功能。

实现了完整的忘记密码/重置密码流程（基于验证码和一次性令牌）。

所有密码存储都使用了bcrypt进行安全哈希。

账户生命周期:

实现了用户自主注销（软删除+状态变更）。

实现了管理员恢复账号和用户自助恢复账号（基于验证码+一次性令牌）的完整流程。

安全加固:

实现了账户自动锁定机制，以防止密码暴力破解。

实现了强制用户下线的管理员功能。

第三部分：管理员模块 (Admin Module)
路由规范: 建立了独立的/admin路由组，实现了权限的集中管理。

核心功能:

实现了管理员查询用户列表（支持分页、筛选、排序）。

实现了管理员修改用户状态（禁用、锁定、恢复正常）。

第四部分：短链接核心与高级管理 (Short Link Core & Advanced)
核心功能: 实现了高性能的短链接创建和跳转 (Redirect) 功能。

高级管理: 实现了分享信息 (shares) 和 标签 (tags) 的管理接口。

核心防护:

缓存: 为Redirect接口建立了包含防穿透、防击穿、防雪崩策略的高性能缓存。

限流: 构建了一套完整的、多维度（全局QPS、IP黑名单、设备、账户、敏感操作）的限流中间件“工具箱”，并已正确应用到各个路由。

第五部分：访问统计数据流水线 (Stats Data Pipeline)
架构: 成功将数据处理流程，从“实时写入DB”，重构为高性能的**“先写Redis，再批量同步到MySQL”**架构。

实时采集 (LogService): 后台消费者工作池 (Worker Pool) 能够异步地接收访问事件，进行数据丰富，并极速写入Redis。

批量同步 (BatchWriterService): 后台定时任务 (Cron Job) 能够定期、原子性地将Redis中的数据（原始日志和统计计数器）批量同步回MySQL的分表和汇总表中。

数据结构: 建立了用于预聚合的汇总表（stats_daily, stats_region_daily等）。

🔜 最后冲刺：剩余的开发计划
正如你所说，在我们完成所有主要功能后，我们将进入最后的收尾阶段。

1. (当前核心任务) 完成统计API接口

目标： 基于已经稳定运行的数据流水线和预聚合汇总表，开发所有面向前端的统计查询接口。

已完成的接口： overview, trend, provinces, cities, devices, sources, logs。

结论： 访问统计模块的第二阶段，已全部完成！

2. (下一阶段) 实现高级功能与数据生命周期管理

任务清单：

POST /export/stats (异步导出CSV)。

GET /admin/stats/global (管理员全局统计)。

实现一个新的定时任务，负责自动清理/归档过期的access_logs_YYYYMM分表。

3. (最终收尾) “边角料”与系统完善

任务清单：

为BatchWriterService实现更健壮的重试/死信队列补偿机制。

在Redirect接口中实现Referer解析，以完成TopSource的统计。

实现真实的短信发送客户端，替换MockSmsClient。

实现/api/v1/policy接口和对应的隐私政策说明。

（可选）暴露/metrics接口用于Prometheus监控。

（可选）为cron.go增加优雅停机 (Graceful Shutdown) 的逻辑。

swagger 更新后执行命令：
swag init -g cmd/main.go -o ./docs