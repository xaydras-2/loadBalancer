import http from "k6/http";
import { sleep, check } from "k6";
import { htmlReport } from "https://raw.githubusercontent.com/benc-uk/k6-reporter/main/dist/bundle.js";

export let options = {
  stages: [
    { duration: "10s", target: 50 },
    { duration: "30s", target: 50 },
    { duration: "10s", target: 0 },
  ],
  thresholds: {
    http_req_duration: ["p(95)<500"],
    http_req_failed: ["rate<0.1"],
  },
};

const BASE_URL = "http://localhost:8080/api/users";

export default function () {
  // 1) POST a new user
  let newUser = {
    Name: `load-test-${__VU}-${__ITER}`,
    Email: `user${__VU}-${__ITER}-${Date.now()}@example.com`,
  };
  let resPost = http.post(BASE_URL, JSON.stringify(newUser), {
    headers: { "Content-Type": "application/json" },
  });
  check(resPost, {
    "POST create: status 200": (r) => r.status === 200,
  });

  // Safely parse created record
  let created;
  try {
    created = resPost.json();
  } catch (e) {
    created = null;
  }
  let id = created && created.id;

  if (id) {
    // 2) GET the user you just created
    let resGet = http.get(`${BASE_URL}/${id}`);
    check(resGet, {
      "GET by id: status 200": (r) => r.status === 200,
    });

    // 3) PUT update
    let resPut = http.put(
      `${BASE_URL}/${id}`,
      JSON.stringify({ Name: `updated-${__VU}-${__ITER}` }),
      { headers: { "Content-Type": "application/json" } }
    );
    check(resPut, {
      "PUT update: status 200": (r) => r.status === 200,
    });

    // 4) DELETE
    let resDel = http.del(`${BASE_URL}/${id}`);
    check(resDel, {
      "DELETE: status 200": (r) => r.status === 200,
    });
  } else {
    console.warn(`Failed to create user in VU ${__VU} iter ${__ITER}`);
  }

  sleep(1);
}

export function handleSummary(data) {
  return {
    "summary.html": htmlReport(data),
    "summary.json": JSON.stringify(data, null, 2),
  };
}
