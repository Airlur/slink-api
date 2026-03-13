import http from 'k6/http';
import { Counter, Trend, Rate } from 'k6/metrics';
import { check, sleep } from 'k6';

// 1. 自定义指标（不变）
const cacheHits = new Counter('cache_hits');
const cacheMisses = new Counter('cache_misses');
const nullCaches = new Counter('null_caches');
const redirectDuration = new Trend('redirect_ms', { unit: 'ms' });
const successRate = new Rate('success_rate');
const limitedRate = new Rate('limited_rate');

// 2. 测试配置：调整sleep间隔，避开单IP限流
export const options = {
  vus: 5,             // 5个VU（对应5个IP）
  duration: '60s',    // 持续60秒
  thresholds: {
    'redirect_ms': ['p(50)<50', 'p(90)<100', 'p(99)<200'],
    'success_rate': ['rate>0.95'],
    'limited_rate': ['rate<0.01'], // 目标：限流比例接近0%
  },
};

// 3. 模拟5个不同IP（1个VU对应1个IP，避免IP浪费）
const SIMULATED_IPS = [
  '192.168.1.101', 
  '192.168.1.102', 
  '192.168.1.103', 
  '192.168.1.104', 
  '192.168.1.105'
];

// 4. 你的短链接列表（不变）
const TEST_SHORT_CODES = [
  'fGKUg3Ng', 'jtsE9954C5s', '123321', '37skKs', '2ad2Ss'
];

// 5. 核心测试逻辑：修复IP伪装+降低频率
export default function () {
  // 每个VU绑定1个固定IP（VU编号从1开始，对应IP数组索引）
  const vuIndex = __VU - 1; // __VU是k6所有版本都支持的变量
  const userIp = SIMULATED_IPS[vuIndex]; // 1个VU对应1个IP

  // 随机选短码
  const randomCode = TEST_SHORT_CODES[Math.floor(Math.random() * TEST_SHORT_CODES.length)];
  const testUrl = `http://localhost:8080/${randomCode}`;

  // 发起请求：用X-Real-IP伪装IP（后端更易识别）
  const start = Date.now();
  const res = http.get(testUrl, {
    headers: {
      'User-Agent': `k6-test-vu-${__VU}`,
      'X-Real-IP': userIp, // 替换为X-Real-IP，优化IP伪装效果
    },
    redirects: 0,
  });
  const duration = Date.now() - start;
  redirectDuration.add(duration);

  // 状态判断
  const cacheStatus = res.headers['X-Cache'] || 'UNKNOWN';
  let isSuccess = false;

  if (res.status === 302) {
    isSuccess = true;
    successRate.add(isSuccess);
    // 统计缓存命中
    if (cacheStatus === 'HIT') {
      cacheHits.add(1);
    } else if (cacheStatus === 'MISS') {
      cacheMisses.add(1);
    }
  } else if (res.status === 403) {
    limitedRate.add(true);
    console.error(`[限流] IP:${userIp}, 短码:${randomCode}, 耗时:${duration}ms`);
  } else if (res.status === 404 && cacheStatus === 'NULL') {
    nullCaches.add(1);
  }

  // 校验：重点检查IP伪装和302状态
  check(res, {
    '302重定向': (r) => r.status === 302,
    'IP伪装头已发送': (r) => r.request.headers['X-Real-IP'] === userIp, // 仅检查是否发送成功
    '缓存标识有效': (r) => ['HIT', 'MISS', 'NULL', 'UNKNOWN'].includes(cacheStatus),
  });

  // 关键：延长sleep到0.7秒，单个IP60秒请求≈84次（≤100次阈值）
  sleep(0.7);
}

// 6. 修复teardown：用iterations替代__ITER
export function teardown() {
  // 用k6内置的iterations变量（所有版本支持），获取总请求数
  const totalRequests = iterations; 
  const totalValid = cacheHits.value + cacheMisses.value;
  const hitRate = totalValid > 0 ? (cacheHits.value / totalValid) * 100 : 0;

  // 安全获取分位值
  const getPercentile = (p) => {
    if (redirectDuration.values && typeof redirectDuration.values.p === 'function') {
      return redirectDuration.values.p(p).toFixed(2);
    }
    return '0.00';
  };

  console.log('\n=====================================');
  console.log('        多IP多用户测试结果（修复后）                ');
  console.log('=====================================');
  console.log(`总请求数: ${totalRequests || 0}`);
  console.log(`有效请求数（302）: ${totalValid || 0}`);
  console.log(`被限流请求数（403）: ${limitedRate.count || 0}`);
  console.log(`缓存命中率: ${hitRate.toFixed(2)}%`);
  console.log(`请求成功比例: ${successRate.value ? (successRate.value * 100).toFixed(2) : '0.00'}%`);
  console.log(`限流比例: ${limitedRate.value ? (limitedRate.value * 100).toFixed(2) : '0.00'}%`);
  console.log(`响应时间 P50: ${getPercentile(50)}ms`);
  console.log(`响应时间 P90: ${getPercentile(90)}ms`);
  console.log('=====================================');
}