import http from 'k6/http';
import { check, sleep } from 'k6';

// Basic k6 test for demonstrating the run functionality
export default function () {
  const response = http.get('https://httpbin.org/get');
  
  check(response, {
    'status is 200': (r) => r.status === 200,
    'response time < 2000ms': (r) => r.timings.duration < 2000,
  });
  
  sleep(1);
}