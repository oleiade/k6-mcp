import http from 'k6/http';
import { check, sleep } from 'k6';

// Test designed for staged load testing
export default function () {
  const response = http.get('https://httpbin.org/get');
  
  check(response, {
    'status is 200': (r) => r.status === 200,
    'response time < 2000ms': (r) => r.timings.duration < 2000,
  });
  
  // Shorter sleep for ramping scenarios
  sleep(0.1);
}