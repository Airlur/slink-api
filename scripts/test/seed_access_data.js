import http from 'k6/http';
import { check, sleep } from 'k6';

const toNumber = (v, def) => {
  const n = Number(v);
  return Number.isFinite(n) ? n : def;
};

const parseCsv = (raw) =>
  (raw || '')
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s.length > 0);

const pick = (arr) => arr[Math.floor(Math.random() * arr.length)];
const randInt = (min, max) => Math.floor(Math.random() * (max - min + 1)) + min;

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const SHORT_CODES = parseCsv(__ENV.SHORT_CODES || 'fGKUg3Ng,jtsE9954C5s,123321,37skKs,2ad2Ss');

if (SHORT_CODES.length === 0) {
  throw new Error('SHORT_CODES cannot be empty, e.g. SHORT_CODES=abc123,def456');
}

// If IP_POOL is provided, use it directly.
// Otherwise auto-generate public IPv4 pool by known domestic/overseas prefixes.
function buildIpPool() {
  const provided = parseCsv(__ENV.IP_POOL || '');
  if (provided.length > 0) {
    return provided;
  }

  const count = Math.max(20, toNumber(__ENV.IP_COUNT, 300));
  const cnRatioRaw = toNumber(__ENV.CN_RATIO, 0.6);
  const cnRatio = Math.max(0, Math.min(1, cnRatioRaw));

  const cnPrefixes = [
    '1.12.24.',
    '14.17.32.',
    '27.38.5.',
    '36.112.14.',
    '39.156.66.',
    '42.81.95.',
    '58.60.188.',
    '60.28.23.',
    '101.6.15.',
    '111.13.101.',
    '113.57.12.',
    '116.62.1.',
    '120.24.64.',
    '123.125.114.',
    '139.196.12.',
    '175.27.228.',
    '180.76.76.',
    '182.254.116.',
    '183.60.92.',
    '202.96.134.',
    '218.30.118.',
    '221.228.32.',
    '223.5.5.',
  ];

  const globalPrefixes = [
    '1.1.1.',
    '8.8.8.',
    '9.9.9.',
    '64.6.64.',
    '76.76.2.',
    '80.80.80.',
    '91.198.174.',
    '94.140.14.',
    '104.16.132.',
    '149.112.112.',
    '151.101.1.',
    '185.228.168.',
    '199.85.126.',
    '208.67.222.',
    '216.58.200.',
  ];

  const set = new Set();
  while (set.size < count) {
    const useCn = Math.random() < cnRatio;
    const prefix = pick(useCn ? cnPrefixes : globalPrefixes);
    set.add(`${prefix}${randInt(1, 254)}`);
  }

  return Array.from(set);
}

const IP_POOL = buildIpPool();

const UA_POOL = parseCsv(
  __ENV.UA_POOL ||
    [
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36',
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 14_4) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15',
      'Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1',
      'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.6367.82 Mobile Safari/537.36',
      'Mozilla/5.0 (iPad; CPU OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1',
    ].join(',')
);

const REFERER_POOL = parseCsv(
  __ENV.REFERER_POOL ||
    [
      '',
      'https://www.google.com/search?q=slink',
      'https://weixin.qq.com/s/demo',
      'https://weibo.com/',
      'https://www.xiaohongshu.com/',
      'https://www.zhihu.com/',
      'https://www.bing.com/search?q=shortlink',
    ].join(',')
);

// Defaults tuned for "a few hundred" requests.
const SLEEP_SECONDS = toNumber(__ENV.SLEEP_SECONDS, 0.35);
const VUS = toNumber(__ENV.VUS, 3);
const DURATION = __ENV.DURATION || '35s';

export const options = {
  vus: VUS,
  duration: DURATION,
  thresholds: {
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(95)<1000'],
  },
};

export function setup() {
  console.log(`seed_access_data: short_codes=${SHORT_CODES.length}, ip_pool=${IP_POOL.length}, ua_pool=${UA_POOL.length}, referer_pool=${REFERER_POOL.length}`);
}

export default function () {
  const code = pick(SHORT_CODES);
  const ip = pick(IP_POOL);
  const ua = pick(UA_POOL);
  const referer = pick(REFERER_POOL);

  const headers = {
    'User-Agent': ua,
    'X-Real-IP': ip,
  };
  if (referer) {
    headers.Referer = referer;
  }

  const res = http.get(`${BASE_URL}/${code}`, {
    headers,
    redirects: 0,
    timeout: '5s',
  });

  check(res, {
    'status is 302 or 403': (r) => r.status === 302 || r.status === 403,
  });

  sleep(SLEEP_SECONDS);
}
