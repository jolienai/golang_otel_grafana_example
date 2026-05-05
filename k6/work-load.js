import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  scenarios: {
    work_endpoint_load: {
      executor: "constant-vus",
      vus: 100,
      duration: "1m",
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.05"],
    http_req_duration: ["p(95)<1000"],
  },
};

const baseURL = __ENV.BASE_URL || "http://localhost:8080";

export default function () {
  const response = http.get(`${baseURL}/work?steps=5`);

  check(response, {
    "work endpoint returns 200": (r) => r.status === 200,
  });

  sleep(1);
}
