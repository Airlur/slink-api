import http from 'k6/http';
import { Counter, Trend, Rate } from 'k6/metrics';
import { check, sleep } from 'k6';

// 1. 自定义指标：修复初始化和统计逻辑
const cacheHits = new Counter('cache_hits');       // 缓存命中（X-Cache: HIT）
const cacheMisses = new Counter('cache_misses');   // 缓存未命中（X-Cache: MISS）
const nullCaches = new Counter('null_caches');     // 空值缓存（X-Cache: NULL）
const redirectDuration = new Trend('redirect_ms', { unit: 'ms' }); // 响应时间
const successRate = new Rate('success_rate');     // 成功重定向（302）比例
const limitedRate = new Rate('limited_rate');      // 被限流（403）比例
const errorRate = new Rate('error_rate');          // 其他错误比例

// 2. 测试配置：严格控制请求频率，避开滑动窗口限流（60秒内≤100次）
export const options = {
  vus: 1,             // 1个虚拟用户（降低并发）
  duration: '60s',    // 测试时长与限流窗口一致
  thresholds: {
    'redirect_ms': ['p(50)<50', 'p(90)<100', 'p(99)<200'],
    'success_rate': ['rate>0.95'],                 // 成功重定向≥95%
    'limited_rate': ['rate<0.01'],                 // 限流比例≤1%（几乎不触发）
    'error_rate': ['rate<0.02'],                   // 其他错误≤2%
  },
};

// 3. 你的短链接列表（替换为数据库中真实存在的短码）
const TEST_SHORT_CODES = [
  'fGKUg3Ng', 
  'jtsE9954C5s', 
  '123321',  
  '37skKs', 
  '2ad2Ss'
];

// 4. 核心测试逻辑：修复状态码判断和缓存统计
export default function () {
  // 随机选择短码
  const randomCode = TEST_SHORT_CODES[Math.floor(Math.random() * TEST_SHORT_CODES.length)];
  const testUrl = `http://localhost:8080/${randomCode}`;

  // 发起请求（禁用自动重定向，捕获原始302响应）
  const start = Date.now();
  const res = http.get(testUrl, {
    headers: { 'User-Agent': 'k6-shortlink-test' },
    redirects: 0,
  });
  const duration = Date.now() - start;
  redirectDuration.add(duration);

  // 状态码和缓存状态处理
  const cacheStatus = res.headers['X-Cache'] || 'UNKNOWN';
  let isSuccess = false;
  let isLimited = false;
  let isError = false;

  if (res.status === 302) {
    // 成功重定向（302）
    isSuccess = true;
    successRate.add(isSuccess);
    // 缓存状态判断
    if (cacheStatus === 'HIT') {
      cacheHits.add(1);
      console.log(`[缓存命中] 短码:${randomCode}, 耗时:${duration}ms`);
    } else if (cacheStatus === 'MISS') {
      cacheMisses.add(1);
      console.log(`[缓存未命中] 短码:${randomCode}, 耗时:${duration}ms`);
    } else {
      errorRate.add(true);
      console.warn(`[异常] 302但缓存标识无效:${cacheStatus}`);
    }

  } else if (res.status === 403) {
    // 被限流（403）
    isLimited = true;
    limitedRate.add(isLimited);
    console.error(`[被限流] 短码:${randomCode}, 状态码:403`);

  } else if (res.status === 404 && cacheStatus === 'NULL') {
    // 短码不存在（404+空值缓存）
    nullCaches.add(1);
    console.log(`[短码不存在] 短码:${randomCode}, 空值缓存生效`);

  } else {
    // 其他错误（如500、400等）
    isError = true;
    errorRate.add(isError);
    console.error(`[其他错误] 短码:${randomCode}, 状态码:${res.status}`);
  }

  // 响应校验
  check(res, {
    '成功重定向返回302': (r) => r.status === 302,
    '302包含Location头': (r) => r.status === 302 && r.headers.Location !== undefined,
    '缓存标识有效': (r) => ['HIT', 'MISS', 'NULL', 'UNKNOWN'].includes(cacheStatus),
    '限流返回403': (r) => r.status === 403,
  });

  // 控制请求频率：1个用户+0.6秒间隔 → 1.67次/秒，60秒共100次（不超过限流阈值）
  sleep(0.6);
}

// 5. 修复后的汇总报告：避免NaN和空值错误
export function teardown() {
  const totalValid = cacheHits.value + cacheMisses.value; // 有效短码请求（302）
  const totalRequests = totalValid + nullCaches.value;    // 总请求数
  const hitRate = totalValid > 0 ? (cacheHits.value / totalValid) * 100 : 0;

  // 安全获取分位值（避免values为null）
  const getPercentile = (p) => {
    if (redirectDuration.values && typeof redirectDuration.values.p === 'function') {
      return redirectDuration.values.p(p).toFixed(2);
    }
    return '0.00';
  };

  console.log('\n=====================================');
  console.log('          测试结果汇总                ');
  console.log('=====================================');
  console.log(`总请求数: ${totalRequests || 0}`);
  console.log(`有效短码请求数（302）: ${totalValid || 0}`);
  console.log(`不存在的短码请求数（404+NULL）: ${nullCaches.value || 0}`);
  console.log(`缓存命中率: ${hitRate.toFixed(2)}%`);
  console.log(`请求成功比例（302）: ${successRate.value ? (successRate.value * 100).toFixed(2) : '0.00'}%`);
  console.log(`被限流比例（403）: ${limitedRate.value ? (limitedRate.value * 100).toFixed(2) : '0.00'}%`);
  console.log(`其他错误比例: ${errorRate.value ? (errorRate.value * 100).toFixed(2) : '0.00'}%`);
  console.log(`响应时间 P50: ${getPercentile(50)}ms`);
  console.log(`响应时间 P90: ${getPercentile(90)}ms`);
  console.log(`响应时间 P99: ${getPercentile(99)}ms`);
  console.log('=====================================');
}