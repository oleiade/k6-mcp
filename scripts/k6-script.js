import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 50 },
    { duration: '1m', target: 50 },
    { duration: '30s', target: 10 },

    { duration: '30s', target: 100 },
    { duration: '1m', target: 100 },
    { duration: '30s', target: 20 },

    { duration: '30s', target: 200 },
    { duration: '1m', target: 200 },
    { duration: '30s', target: 40 },

    { duration: '30s', target: 300 },
    { duration: '1m', target: 300 },
    { duration: '30s', target: 60 },

    { duration: '30s', target: 400 },
    { duration: '1m', target: 400 },
    { duration: '30s', target: 80 },

    { duration: '30s', target: 500 },
    { duration: '2m', target: 500 },
    { duration: '2m', target: 0 },
  ],
  thresholds: {
    http_req_failed: ['rate<0.05'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'https://quickpizza.grafana.com';

export default function () {
  const res = http.get(BASE_URL);
  check(res, {
    'status is 200': (r) => r.status === 200,
  });
  sleep(1);
}


