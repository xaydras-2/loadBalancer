import http from 'k6/http';
import { sleep, check } from 'k6';
import { htmlReport } from "https://raw.githubusercontent.com/benc-uk/k6-reporter/main/dist/bundle.js";


// Test options
export let options = {
  stages: [
    { duration: '10s', target: 50 }, // ramp up to 50 virtual users over 30s
    { duration: '30s',  target: 50 }, // stay at 50 VUs for 1m
    { duration: '10s', target: 0 },  // ramp down to 0 over 30s
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% of requests must finish in <500ms
  },
};

// The “default” function runs once per VU in a loop
export default function () {
  let res = http.get('http://localhost:8080/api/users');
  check(res, {
    'status is 200': (r) => r.status === 200,
  });
  sleep(1); // each VU waits 1s between iterations
}

export function handleSummary(data) {
  // this will write summary.html into your working directory
  return {
    "summary.html": htmlReport(data),
  };
}
