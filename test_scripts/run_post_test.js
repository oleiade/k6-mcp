import http from 'k6/http';
import { check } from 'k6';

// Test with POST requests and JSON payloads
export default function () {
  const payload = JSON.stringify({
    name: 'k6 test',
    timestamp: new Date().toISOString(),
    data: {
      test: true,
      value: Math.random()
    }
  });
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };
  
  const response = http.post('https://httpbin.org/post', payload, params);
  
  check(response, {
    'status is 200': (r) => r.status === 200,
    'json response': (r) => {
      try {
        const json = JSON.parse(r.body);
        return json.json && json.json.name === 'k6 test';
      } catch {
        return false;
      }
    },
    'response time < 2000ms': (r) => r.timings.duration < 2000,
  });
}