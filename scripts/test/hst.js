import http from 'k6/http';
import { Counter, Trend, Rate } from 'k6/metrics';
import { check, sleep } from 'k6';

// 1. 自定义指标（确保变量名与阈值名完全一致）
const cacheHits = new Counter('cache_hits');
const cacheMisses = new Counter('cache_misses');
const redirectDuration = new Trend('redirect_ms', { unit: 'ms' });
const successRate = new Rate('success_rate');
const globalLimitedRate = new Rate('global_limited_rate');
const ipLimitedRate = new Rate('ip_limited_rate'); // 修正：变量名改为小写分隔（与阈值一致）
const errorRate = new Rate('error_rate');

// 2. 测试配置（阈值名与指标名严格一致）
export const options = {
  stages: [
    { duration: '10s', target: 100 },  // 30秒升到80 VU（QPS≈1600）
    { duration: '40s', target: 120 }, // 稳定运行60秒，观察P99是否稳定
    // { duration: '30s', target: 150 }, // 30秒升到150 VU（QPS≈3000）
    { duration: '10s', target: 0 },   // 收尾
  ],
  thresholds: {
    // 'redirect_ms': ['p(50)<100', 'p(90)<200', 'p(99)<500'],
    // 'success_rate': ['rate>0.80'],
    // 'global_limited_rate': ['rate<0.30'],
    // 'ip_limited_rate': ['rate<0.01'], // 修正：指标名改为小写分隔（与变量一致）
    // 'error_rate': ['rate<0.05'],

    // 放宽阈值，允许性能波动（重点看“何时出现失败”）
    'redirect_ms': ['p(50)<30', 'p(90)<50', 'p(99)<80'],
    'success_rate': ['rate>0.99'],    // 成功比例低于99%即视为触顶
    'http_req_failed': ['rate<0.01'], // 失败率超过1%即停止加压
  },
};

// 3. 有效短码列表（仅使用你提供的真实短码）
const VALID_SHORT_CODES = [
  'jtsE9954C5s', 
  '123321', 
  '37skKs', 
  '2ad2Ss', 
  '77Mm8Me4elI', 
  'fGKUg3Ng'
];

// 4. 模拟30个IP
const SIMULATED_IPS = Array.from({ length: 30 }, (_, i) => `192.168.2.${100 + i}`);

// 5. 全局变量：统计总请求数
let totalRequests = 0;

// 6. 核心测试逻辑
export default function () {
  totalRequests++;

  // 随机选择有效短码和IP
  const randomCode = VALID_SHORT_CODES[Math.floor(Math.random() * VALID_SHORT_CODES.length)];
  const randomIp = SIMULATED_IPS[Math.floor(Math.random() * SIMULATED_IPS.length)];
  const testUrl = `http://localhost:8080/${randomCode}`;

  // 发起请求
  const start = Date.now();
  const res = http.get(testUrl, {
    headers: {
      'User-Agent': `k6-high-concurrency-${__VU}`,
      'X-Real-IP': randomIp,
    },
    redirects: 0,
  });
  const duration = Date.now() - start;
  redirectDuration.add(duration);

  // 状态判断
  const cacheStatus = res.headers['X-Cache'] || 'UNKNOWN';
  if (res.status === 302) {
    successRate.add(true);
    cacheStatus === 'HIT' ? cacheHits.add(1) : cacheMisses.add(1);
  } else if (res.status === 403) {
    const limitType = res.headers['X-Limit-Type'] || 'global';
    limitType === 'ip' ? ipLimitedRate.add(true) : globalLimitedRate.add(true);
  } else {
    errorRate.add(true);
    console.error(`[异常] 有效短码${randomCode}返回状态码:${res.status}`);
  }

  // 响应校验
  check(res, {
    '有效短码响应合法': (r) => [302, 403].includes(r.status),
    '缓存标识有效': (r) => ['HIT', 'MISS'].includes(cacheStatus),
  });

  // 控制请求频率
  sleep(0.05); // 150 VU × 0.05秒 ≈ 3000 QPS（足够逼出瓶颈）
}

// 7. 测试报告
export function teardown() {
  const totalValid = cacheHits.value + cacheMisses.value;
  const hitRate = totalValid > 0 ? (cacheHits.value / totalValid) * 100 : 0;
  const totalDuration = 80;
  const globalQPS = totalRequests > 0 ? (totalRequests / totalDuration).toFixed(2) : '0';

  const getPercentile = (p) => {
    return redirectDuration.values?.p ? redirectDuration.values.p(p).toFixed(2) : '0.00';
  };

  console.log('\n=====================================');
  console.log('          高并发测试结果（真实短码）         ');
  console.log('=====================================');
  console.log(`总请求数: ${totalRequests}`);
  console.log(`全局实际QPS: ${globalQPS}`);
  console.log(`成功访问数（302）: ${totalValid || 0}`);
  console.log(`缓存命中率: ${hitRate.toFixed(2)}%`);
  console.log(`成功访问比例: ${(successRate.value * 100).toFixed(2)}%`);
  console.log(`被全局限流比例: ${(globalLimitedRate.value * 100).toFixed(2)}%`);
  console.log(`被IP限流比例: ${(ipLimitedRate.value * 100).toFixed(2)}%`);
  console.log(`异常错误比例（有效短码）: ${(errorRate.value * 100).toFixed(2)}%`);
  console.log(`响应时间 P50: ${getPercentile(50)}ms`);
  console.log(`响应时间 P90: ${getPercentile(90)}ms`);
  console.log(`响应时间 P99: ${getPercentile(99)}ms`);
  console.log('=====================================');
}