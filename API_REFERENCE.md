# 短链接服务 API 接口文档

本文档基于 SLINK-go 后端 Swagger 定义生成，旨在为SLINK——短链接前端WEB项目的开发提供清晰的接口参考。

## 📊 接口概览

**接口总数**: 47 个

**模块统计**:
- **验证码模块**: 2 个
- **用户模块**: 15 个
- **短链接模块**: 8 个
- **标签模块**: 3 个
- **统计模块**: 9 个
- **分享模块**: 7 个
- **管理员-用户管理**: 3 个

**鉴权说明**:
- 🔒 **需要鉴权**: 请求 Header 需携带 `Authorization: Bearer {token}`
- 🔓 **公开接口**: 无需携带 Token
- 🔓/🔒 **可选鉴权**: 可带可不带，逻辑会有所不同（如创建短链接）

---

## 🟢 1. 验证码模块

### 1.1 发送验证码
> 向指定邮箱或手机发送验证码 (注册/登录/重置密码等)

- **URL**: `/api/v1/captcha/send`
- **Method**: `POST`
- **Auth**: 🔓 公开

**请求参数 (Body)**:
```json
{
  "account": "user@example.com", // [必填] 手机号或邮箱
  "scene": "register",           // [必填] 场景: register(注册)/login(登录)/reset_pwd(重置密码)
  "type": "email"                // [可选] 类型: sms/email, 默认 email
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "发送成功",
  "data": {
    "expire_second": 300,      // 验证码有效期（秒）
    "next_send_second": 60     // 下次发送冷却时间（秒）
  }
}
```

### 1.2 验证验证码
> 校验用户输入的验证码是否正确

- **URL**: `/api/v1/captcha/verify`
- **Method**: `POST`
- **Auth**: 🔓 公开

**请求参数 (Body)**:
```json
{
  "account": "user@example.com", // [必填]
  "captcha": "123456",           // [必填] 6位验证码
  "scene": "register"            // [必填]
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "验证成功",
  "data": null
}
```

---

## 👤 2. 用户模块

### 2.1 用户注册
> 新用户注册，需先获取验证码

- **URL**: `/api/v1/register`
- **Method**: `POST`
- **Auth**: 🔓 公开

**请求参数 (Body)**:
```json
{
  "username": "user123",         // [必填] 3-32位
  "password": "password123",     // [必填] 6-32位
  "account": "user@example.com", // [必填] 用于接收验证码的账号
  "captcha": "123456",           // [必填]
  "type": "email"                // [可选] email/phone
}
```

### 2.2 用户登录
> 用户登录获取访问令牌

- **URL**: `/api/v1/login`
- **Method**: `POST`
- **Auth**: 🔓 公开

**请求参数 (Body)**:
```json
{
  "username": "user123",    // [必填]
  "password": "password123" // [必填]
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "登录成功",
  "data": {
    "accessToken": "eyJhbG...",   // 短期访问令牌
    "refreshToken": "d8e1...",    // 长期刷新令牌
    "user": {
      "id": 1,
      "username": "user123",
      "nickname": "Nick",
      "avatar": "https://example.com/avatar.jpg",
      "email": "user@example.com",
      "role": 1,
      "status": 1
    }
  }
}
```

### 2.3 刷新 Token
> 使用 Refresh Token 换取新的 Access Token

- **URL**: `/api/v1/users/token/refresh`
- **Method**: `POST`
- **Auth**: 🔓 公开

**请求参数 (Body)**:
```json
{
  "refreshToken": "d8e1..." // [必填]
}
```

### 2.4 获取当前用户信息
- **URL**: `/api/v1/users/{id}`
- **Method**: `GET`
- **Auth**: 🔒 需要鉴权

### 2.5 更新用户信息
- **URL**: `/api/v1/users/{id}`
- **Method**: `PUT`
- **Auth**: 🔒 需要鉴权

**请求参数 (Body)**:
```json
{
  "nickname": "New Nick",   // [可选]
  "avatar": "http://...",   // [可选]
  "email": "new@ex.com",    // [可选]
  "phone": "13800000000"    // [可选]
}
```

### 2.6 修改密码
- **URL**: `/api/v1/users/{id}/reset`
- **Method**: `PUT`
- **Auth**: 🔒 需要鉴权

**请求参数 (Body)**:
```json
{
  "oldPassword": "oldpass", // [必填]
  "newPassword": "newpass"  // [必填]
}
```

### 2.7 检查用户是否存在
> 检查用户名或邮箱是否已被注册

- **URL**: `/api/v1/users/check`
- **Method**: `GET`
- **Auth**: 🔓 公开

**请求参数 (Query)**:
- `username`: string (可选)
- `email`: string (可选)

**响应示例**:
```json
{
  "code": 0,
  "data": {
    "exists": true
  }
}
```

### 2.8 退出登录
- **URL**: `/api/v1/users/logout`
- **Method**: `POST`
- **Auth**: 🔒 需要鉴权

### 2.9 忘记密码 - 发送验证码
- **URL**: `/api/v1/users/password/forgot`
- **Method**: `POST`
- **Auth**: 🔓 公开

**请求参数 (Body)**:
```json
{
  "account": "user@example.com", // [必填]
  "type": "email"                // [可选]
}
```

### 2.10 忘记密码 - 验证验证码
> 验证通过后返回 resetToken 用于重置密码

- **URL**: `/api/v1/users/password/verify`
- **Method**: `POST`
- **Auth**: 🔓 公开

**请求参数 (Body)**:
```json
{
  "account": "user@example.com",
  "captcha": "123456",
  "type": "email"
}
```

**响应示例**:
```json
{
  "code": 0,
  "data": {
    "resetToken": "token_for_reset_password" // 用于下一步
  }
}
```

### 2.11 重置密码
- **URL**: `/api/v1/users/password/reset`
- **Method**: `POST`
- **Auth**: 🔓 公开

**请求参数 (Body)**:
```json
{
  "resetToken": "token_from_verify_step", // [必填]
  "password": "newPassword123"            // [必填]
}
```

### 2.12 - 2.15 账号恢复流程
*(接口类似于忘记密码流程，路径为 `/users/recovery/*`，参数类似，此处省略)*

---

## 🔗 3. 短链接模块

### 3.1 创建短链接
> 游客可直接调用；登录用户带 Token 调用可享更多功能（自定义短码、有效期等）。

- **URL**: `/api/v1/shortlinks`
- **Method**: `POST`
- **Auth**: 🔓/🔒 可选鉴权

**请求参数 (Body)**:
```json
{
  "originalUrl": "https://google.com", // [必填] 原始链接
  "shortCode": "custom123",            // [可选] 自定义短码 (仅登录用户)
  "expiresIn": "7d"                    // [可选] 有效期 (1h/24h/7d/30d/90d/1y/never)
}
```

**响应示例**:
```json
{
  "code": 0,
  "data": {
    "shortUrl": "http://d.cn/custom123",
    "shortCode": "custom123",
    "originalUrl": "https://google.com",
    "expireAt": "2025-12-17T00:00:00Z",
    "status": 1
  }
}
```

### 3.2 短链接重定向
> 核心跳转接口

- **URL**: `/api/v1/{shortCode}`
- **Method**: `GET`
- **Auth**: 🔓 公开
- **Response**: 302 Redirect 或 404 HTML 页面

### 3.3 获取我的短链接列表
> 分页获取当前用户的短链接

- **URL**: `/api/v1/shortlinks/my`
- **Method**: `GET`
- **Auth**: 🔒 需要鉴权

**请求参数 (Query)**:
- `page`: int (默认1)
- `limit`: int (默认20)
- `tag`: string (筛选标签名)
- `sort_by`: string (排序字段)

**响应示例**:
```json
{
  "code": 0,
  "data": {
    "data": [
      {
        "shortCode": "AbCd12",
        "originalUrl": "https://...",
        "clickCount": 100,
        "tags": ["推广", "双11"],
        "created_at": "2025-12-10T12:00:00Z",
        "share": { "title": "...", "desc": "...", "image": "..." }
      }
    ],
    "pagination": {
      "page": 1,
      "limit": 20,
      "total": 50
    }
  }
}
```

### 3.4 获取短链接详情
- **URL**: `/api/v1/shortlinks/{short_code}`
- **Method**: `GET`
- **Auth**: 🔒 需要鉴权

### 3.5 更新短链接信息
- **URL**: `/api/v1/shortlinks/{short_code}`
- **Method**: `PUT`
- **Auth**: 🔒 需要鉴权

**请求参数 (Body)**:
```json
{
  "originalUrl": "...",   // [可选]
  "status": 1,            // [可选] 0失效/1有效
  "expiresIn": "30d"      // [可选]
}
```

### 3.6 更新短链接状态
- **URL**: `/api/v1/shortlinks/{short_code}/status`
- **Method**: `PATCH`
- **Auth**: 🔒 需要鉴权

**请求参数 (Body)**:
```json
{ "status": 0 } // 0:禁用, 1:启用
```

### 3.7 延长有效期
- **URL**: `/api/v1/shortlinks/{short_code}/expiration`
- **Method**: `PATCH`
- **Auth**: 🔒 需要鉴权

**请求参数 (Body)**:
```json
{ "expiresIn": "1y" }
```

### 3.8 删除短链接
- **URL**: `/api/v1/shortlinks/{short_code}`
- **Method**: `DELETE`
- **Auth**: 🔒 需要鉴权

---

## 🏷️ 4. 标签模块

### 4.1 获取我的标签列表
- **URL**: `/api/v1/tags`
- **Method**: `GET`
- **Auth**: 🔒 需要鉴权

**响应示例**:
```json
{
  "code": 0,
  "data": {
    "tags": ["活动", "社群", "推广"]
  }
}
```

### 4.2 添加标签
- **URL**: `/api/v1/shortlinks/{short_code}/tags`
- **Method**: `POST`
- **Auth**: 🔒 需要鉴权

**请求参数 (Body)**:
```json
{ "tagName": "2025活动" }
```

### 4.3 移除标签
- **URL**: `/api/v1/shortlinks/{short_code}/tags`
- **Method**: `DELETE`
- **Auth**: 🔒 需要鉴权

**请求参数 (Body)**:
```json
{ "tagName": "2025活动" }
```

---

## 📈 5. 统计模块

> 所有统计接口均需鉴权，路径包含 `/shortlinks/{short_code}/stats/...`

### 5.1 概览 (Overview)
- **URL**: `/api/v1/shortlinks/{short_code}/stats/overview`
- **Method**: `GET`
- **Response**: 包含总点击、今日点击、Top地区等。

### 5.2 趋势 (Trend)
- **URL**: `/api/v1/shortlinks/{short_code}/stats/trend`
- **Method**: `GET`
- **Params**: `granularity` (day/hour), `start_date`, `end_date`

### 5.3 来源 (Sources)
- **URL**: `/api/v1/shortlinks/{short_code}/stats/sources`
- **Method**: `GET`

### 5.4 访问日志 (Logs)
- **URL**: `/api/v1/shortlinks/{short_code}/stats/logs`
- **Method**: `GET`
- **Params**: `page`, `page_size`

### 5.5 其他统计
- **省份分布**: `/api/v1/shortlinks/{short_code}/stats/provinces`
- **城市分布**: `/api/v1/shortlinks/{short_code}/stats/cities?province=xxx`
- **设备分布**: `/api/v1/shortlinks/{short_code}/stats/devices?dimension=os` (或 browser/device_type)

### 5.6 全局统计 (管理员)
- **URL**: `/api/v1/admin/stats/global`
- **Method**: `GET`
- **Auth**: 🔒 管理员权限

---

## 📤 6. 分享模块

> 管理短链接在社交媒体分享时的卡片信息（Open Graph 协议支持）

### 6.1 获取分享信息
- **URL**: `/api/v1/shortlinks/{short_code}/share`
- **Method**: `GET`
- **Auth**: 🔒 需要鉴权

### 6.2 设置分享信息
- **URL**: `/api/v1/shortlinks/{short_code}/share`
- **Method**: `PUT`
- **Auth**: 🔒 需要鉴权

**请求参数 (Body)**:
```json
{
  "shareTitle": "超棒的活动！",
  "shareDesc": "点击查看详情...",
  "shareImage": "https://example.com/cover.jpg"
}
```

---

## 👮 7. 管理员-用户管理

### 7.1 获取用户列表
- **URL**: `/api/v1/admin/users`
- **Method**: `GET`
- **Auth**: 🔒 管理员权限
- **Params**: `page`, `pageSize`, `username` (模糊搜索), `status`

### 7.2 更新用户状态
- **URL**: `/api/v1/admin/users/{id}/status`
- **Method**: `PUT`
- **Auth**: 🔒 管理员权限

**请求参数 (Body)**:
```json
{ "status": 2 } // 1:正常, 2:禁用, 3:锁定
```

### 7.3 强制下线
- **URL**: `/api/v1/admin/users/{id}/session`
- **Method**: `DELETE`
- **Auth**: 🔒 管理员权限
