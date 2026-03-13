# scripts/test 测试脚本说明

本目录用于短链接访问测试与测试数据造数，包含 `k6` 脚本与 `bat` 辅助脚本。

## 前置条件

1. 后端服务已启动（默认 `http://localhost:8080`）。
2. Redis/MySQL 正常可用，定时任务在运行。
3. 使用的短码必须真实存在且可正常跳转（302）。
4. 已安装 `k6`（命令 `k6 version` 可用）。

## 脚本用途

1. `seed_access_data.js`
   - 推荐使用的造数脚本。
   - 支持伪造不同 IP、设备 UA、来源 Referer。
   - 支持自动生成大量国内外公网 IP（默认可配到几百个）。

2. `st.js`
   - 早期基础访问测试脚本（低并发，验证 302/缓存/限流）。

3. `stip.js`
   - 早期多 IP 多用户测试脚本（中并发）。

4. `hst.js`
   - 早期高并发压测脚本（偏性能压测）。

5. `sl.bat`
   - 使用 curl 连续请求单个短链（固定参数）。

6. `s2.bat`
   - 使用多个 cmd 进程做并发 curl 请求。

7. `t.html`
   - 历史测试页面，和当前短链接主流程无直接关系。

## 推荐造数方式（几百次 + 几百个 IP）

下面命令会固定 300 次请求，并自动生成 300 个国内外混合 IP：

```powershell
k6 run --vus 3 --iterations 300 `
  -e BASE_URL=http://localhost:8080 `
  -e SHORT_CODES=52KD2a,nvGQWJQ0,lV86cWyd,Doubao,13dsw4 `
  -e IP_COUNT=300 `
  -e CN_RATIO=0.6 `
  -e REFERER_POOL=https://weixin.qq.com/s/demo,https://weibo.com/,https://www.google.com/search?q=slink,https://www.zhihu.com/ `
  scripts/test/seed_access_data.js
```

说明：

1. `--iterations 300`：精确总请求数为 300 次。
2. `IP_COUNT=300`：自动生成 300 个公网 IP。
3. `CN_RATIO=0.6`：约 60% 国内 IP、40% 海外 IP。
4. `SHORT_CODES`：逗号分隔，不要在中间断行。

## seed_access_data.js 可用环境变量

1. `BASE_URL`：后端地址，默认 `http://localhost:8080`。
2. `SHORT_CODES`：短码列表，逗号分隔（必填，需真实存在）。
3. `IP_POOL`：自定义 IP 列表（若传入则优先使用，不再自动生成）。
4. `IP_COUNT`：自动生成 IP 数量（未传 `IP_POOL` 时生效，默认 300）。
5. `CN_RATIO`：自动生成 IP 的国内占比（0~1，默认 0.6）。
6. `REFERER_POOL`：来源列表，逗号分隔。
7. `UA_POOL`：User-Agent 列表，逗号分隔。
8. `VUS`：并发虚拟用户数（脚本默认 3）。
9. `DURATION`：测试时长（脚本默认 `35s`）。
10. `SLEEP_SECONDS`：每次请求后休眠秒数（脚本默认 0.35）。

## 造数后验证

1. 等待约 10~20 秒（默认有定时批量落库）。
2. 在前端短链详情页查看趋势、地域、设备、来源数据是否变化。
3. 或调用统计接口验证：

```powershell
curl -H "Authorization: Bearer <token>" "http://localhost:8080/api/v1/shortlinks/<short_code>/stats/provinces"
curl -H "Authorization: Bearer <token>" "http://localhost:8080/api/v1/shortlinks/<short_code>/stats/devices?dimension=os"
curl -H "Authorization: Bearer <token>" "http://localhost:8080/api/v1/shortlinks/<short_code>/stats/sources"
```

## 注意事项

1. 若短码不存在，请求会变成 404，不会产生有效统计。
2. 如果你只想造几十次，请把 `--iterations` 改小（如 80、100）。
3. 当前项目限流阈值较高，几百次造数通常不会触发限流。
