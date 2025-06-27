import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Load test with multiple endpoints and metrics
const errorRate = new Rate('errors');

export default function () {
  // Test multiple endpoints
  const endpoints = [
    'https://httpbin.org/get',
    'https://httpbin.org/status/200',
    'https://httpbin.org/delay/1'
  ];
  
  endpoints.forEach(url => {
    const response = http.get(url);
    
    const result = check(response, {
      'status is 200': (r) => r.status === 200,
      'response time < 3000ms': (r) => r.timings.duration < 3000,
    });
    
    errorRate.add(!result);
  });
  
  sleep(0.5);
}