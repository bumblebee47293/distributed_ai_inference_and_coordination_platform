import http from "k6/http";
import { check, sleep } from "k6";
import { Rate } from "k6/metrics";

// Custom metrics
const errorRate = new Rate("errors");

// Test configuration
export const options = {
  stages: [
    { duration: "30s", target: 50 }, // Ramp up to 50 users
    { duration: "1m", target: 100 }, // Ramp up to 100 users
    { duration: "2m", target: 100 }, // Stay at 100 users
    { duration: "30s", target: 200 }, // Spike to 200 users
    { duration: "1m", target: 200 }, // Stay at 200 users
    { duration: "30s", target: 0 }, // Ramp down to 0 users
  ],
  thresholds: {
    http_req_duration: ["p(95)<100", "p(99)<150"], // 95% < 100ms, 99% < 150ms
    http_req_failed: ["rate<0.01"], // Error rate < 1%
    errors: ["rate<0.01"],
  },
};

const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";

export default function () {
  // Real-time inference request
  const payload = JSON.stringify({
    model: "resnet18",
    version: "v1",
    input: {
      image: "base64_encoded_image_data_placeholder",
    },
  });

  const params = {
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer demo-token",
    },
  };

  const response = http.post(`${BASE_URL}/v1/infer`, payload, params);

  // Validate response
  const success = check(response, {
    "status is 200": (r) => r.status === 200,
    "response has request_id": (r) =>
      JSON.parse(r.body).request_id !== undefined,
    "response has prediction": (r) =>
      JSON.parse(r.body).prediction !== undefined,
    "latency is acceptable": (r) => r.timings.duration < 200,
  });

  errorRate.add(!success);

  sleep(1);
}

export function handleSummary(data) {
  return {
    stdout: textSummary(data, { indent: " ", enableColors: true }),
    "loadtest-results.json": JSON.stringify(data),
  };
}

function textSummary(data, options) {
  const indent = options.indent || "";
  const enableColors = options.enableColors || false;

  let summary = "\n";
  summary += `${indent}Scenarios: ${data.metrics.scenarios ? Object.keys(data.metrics.scenarios).length : 0}\n`;
  summary += `${indent}Checks: ${data.metrics.checks ? data.metrics.checks.values.passes : 0} passed, ${data.metrics.checks ? data.metrics.checks.values.fails : 0} failed\n`;
  summary += `${indent}HTTP Requests: ${data.metrics.http_reqs ? data.metrics.http_reqs.values.count : 0}\n`;
  summary += `${indent}HTTP Request Duration (p95): ${data.metrics.http_req_duration ? data.metrics.http_req_duration.values["p(95)"].toFixed(2) : 0}ms\n`;
  summary += `${indent}HTTP Request Duration (p99): ${data.metrics.http_req_duration ? data.metrics.http_req_duration.values["p(99)"].toFixed(2) : 0}ms\n`;
  summary += `${indent}Error Rate: ${data.metrics.errors ? (data.metrics.errors.values.rate * 100).toFixed(2) : 0}%\n`;

  return summary;
}
