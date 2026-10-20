import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  buildAccessUrl,
  getProjectRuntimeStatus,
  getContainerStatusColor,
  getContainerStatusLabel,
  getBuildStatusColor,
  getBuildStatusLabel,
  normalizeRuntimeLog,
  RECORDS_POLL_INTERVAL_MS,
} from "./projectDetailHelpers";

describe("buildAccessUrl", () => {
  it("builds http URL with port and no route for a running-style project", () => {
    const url = buildAccessUrl(
      { use_https: false, port: 6012, frontend_route: "/" },
      "app.example.com",
    );
    assert.equal(url, "http://app.example.com:6012");
  });

  it("uses https and joins a non-root frontend_route", () => {
    const url = buildAccessUrl(
      { use_https: true, port: 8443, frontend_route: "login" },
      "phish.local",
    );
    assert.equal(url, "https://phish.local:8443/login");
  });

  it("defaults port to 8080 when missing", () => {
    const url = buildAccessUrl({ frontend_route: "/app" }, "localhost");
    assert.equal(url, "http://localhost:8080/app");
  });

  it("prefers a running agent deployment runtime_url over host:port", () => {
    const url = buildAccessUrl(
      {
        use_https: false,
        port: 20001,
        frontend_route: "/",
        deployments: [
          {
            actual_status: "running",
            runtime_url: "http://agent.example.com:20000/",
          },
        ],
      },
      "control.example.com",
    );
    assert.equal(url, "http://agent.example.com:20000");
  });

  it("falls back to host:port when deployments have no runtime_url", () => {
    const url = buildAccessUrl(
      {
        port: 20001,
        deployments: [{ actual_status: "pending", runtime_url: "" }],
      },
      "control.example.com",
    );
    assert.equal(url, "http://control.example.com:20001");
  });
});

describe("getProjectRuntimeStatus", () => {
  it("uses container_status when there are no deployments", () => {
    assert.equal(
      getProjectRuntimeStatus({ container_status: "running" }),
      "running",
    );
    assert.equal(
      getProjectRuntimeStatus({ container_status: "stopped" }),
      "stopped",
    );
  });

  it("prefers running agent deployment over stopped container_status", () => {
    assert.equal(
      getProjectRuntimeStatus({
        container_status: "stopped",
        deployments: [
          {
            actual_status: "running",
            runtime_url: "http://agent.example.com:20000/",
          },
        ],
      }),
      "running",
    );
  });

  it("maps pending agent deployment to starting", () => {
    assert.equal(
      getProjectRuntimeStatus({
        container_status: "stopped",
        deployments: [{ actual_status: "pending" }],
      }),
      "starting",
    );
  });
});

describe("status maps", () => {
  it("maps container status to color and label", () => {
    assert.equal(getContainerStatusColor("running"), "green");
    assert.equal(getContainerStatusLabel("running"), "running");
    assert.equal(getContainerStatusColor("starting"), "gold");
    assert.equal(getContainerStatusLabel("stopping"), "stopping");
    assert.equal(getContainerStatusColor("nope"), "default");
    assert.equal(getContainerStatusLabel("nope"), "unknown");
  });

  it("maps build status to color and label", () => {
    assert.equal(getBuildStatusColor("success"), "green");
    assert.equal(getBuildStatusLabel("building"), "building");
    assert.equal(getBuildStatusColor("failed"), "red");
    assert.equal(getBuildStatusLabel("pending"), "pending");
    assert.equal(getBuildStatusColor("weird"), "default");
    assert.equal(getBuildStatusLabel("weird"), "unknown");
  });
});

describe("normalizeRuntimeLog", () => {
  it("strips ANSI escapes and normalizes CR noise", () => {
    const raw =
      "\u001b[31mERROR\u001b[0m line one\r\n" + "line two\r" + "line three\u0007";
    const cleaned = normalizeRuntimeLog(raw);
    assert.equal(cleaned, "ERROR line one\nline two\nline three");
    assert.equal(cleaned.includes("\u001b"), false);
    assert.equal(cleaned.includes("\r"), false);
  });

  it("strips docker-style leading timestamps", () => {
    const raw = "2026-08-28T03:15:01.123456Z hello from container\nnext";
    assert.equal(normalizeRuntimeLog(raw), "hello from container\nnext");
  });

  it("returns empty string for falsy input", () => {
    assert.equal(normalizeRuntimeLog(null), "");
    assert.equal(normalizeRuntimeLog(""), "");
  });
});

describe("records poll interval", () => {
  it("exports a positive interval used by Records tab", () => {
    assert.equal(typeof RECORDS_POLL_INTERVAL_MS, "number");
    assert.ok(RECORDS_POLL_INTERVAL_MS >= 5000);
    assert.ok(RECORDS_POLL_INTERVAL_MS <= 60000);
  });
});
