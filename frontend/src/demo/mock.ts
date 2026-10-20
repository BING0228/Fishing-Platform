import type { AxiosAdapter, AxiosResponse, InternalAxiosRequestConfig } from "axios";
import { DEMO_TOKEN, DEMO_USER } from "./config";

type Json = Record<string, unknown> | unknown[] | string | number | boolean | null;

const now = Date.now();
const iso = (minutesAgo: number) =>
  new Date(now - minutesAgo * 60_000).toISOString();

const demoProjects = [
  {
    id: 1,
    name: "q4-awareness",
    description: "Q4 security awareness landing page",
    run_mode: "local",
    frontend_route: "/",
    container_route: "/api/submit",
    use_https: false,
    has_ssl_cert: false,
    container_status: "running",
    docker_image: "",
    container_id: "local:demo-1",
    port: 8090,
    login_url: "https://example.com/q4-login",
    qr_relay_id: null,
    build_status: "success",
    deployment_revision: 2,
    created_at: iso(1440),
    updated_at: iso(12),
    robot_ids: [1],
    robots: [{ id: 1, name: "Security Operations" }],
    credential_count: 18,
    agent_ids: [],
    deployments: [],
  },
  {
    id: 2,
    name: "finance-login",
    description: "Finance portal simulation with agent deployment",
    run_mode: "docker",
    frontend_route: "/portal",
    container_route: "/api/submit",
    use_https: false,
    has_ssl_cert: false,
    container_status: "building",
    docker_image: "spearhook-finance-login:latest",
    container_id: "",
    port: 8091,
    login_url: "https://example.com/finance",
    qr_relay_id: null,
    build_status: "building",
    deployment_revision: 4,
    created_at: iso(2880),
    updated_at: iso(5),
    robot_ids: [1],
    robots: [{ id: 1, name: "Security Operations" }],
    credential_count: 6,
    agent_ids: [1, 2],
    deployments: [
      {
        id: 11,
        agent_id: 1,
        agent: { name: "edge-sgp-01" },
        actual_status: "running",
        runtime_url: "http://edge-sgp.example.com:20001",
        deployed_revision: 4,
        runtime_port: 20001,
      },
      {
        id: 12,
        agent_id: 2,
        agent: { name: "edge-fra-02" },
        actual_status: "running",
        runtime_url: "http://edge-fra.example.com:20002",
        deployed_revision: 4,
        runtime_port: 20002,
      },
    ],
  },
  {
    id: 3,
    name: "helpdesk-portal",
    description: "IT helpdesk awareness simulation",
    run_mode: "local",
    frontend_route: "/",
    container_route: "/api/submit",
    use_https: false,
    has_ssl_cert: false,
    container_status: "running",
    docker_image: "",
    container_id: "local:demo-3",
    port: 8092,
    login_url: "https://example.com/helpdesk",
    qr_relay_id: null,
    build_status: "success",
    deployment_revision: 1,
    created_at: iso(4320),
    updated_at: iso(35),
    robot_ids: [],
    robots: [],
    credential_count: 3,
    agent_ids: [],
    deployments: [],
  },
];

const demoAgents = [
  {
    id: 1,
    name: "edge-sgp-01",
    status: "online",
    agent_id: "agent-sgp-01",
    hostname: "edge-sgp-01",
    ip_address: "203.0.113.42",
    version: "1.8.2",
    os: "linux",
    arch: "amd64",
    capabilities: ["docker", "qr-relay"],
    tags: ["production", "apac"],
    deployment_count: 2,
    last_seen_at: iso(0.4),
  },
  {
    id: 2,
    name: "edge-fra-02",
    status: "online",
    agent_id: "agent-fra-02",
    hostname: "edge-fra-02",
    ip_address: "198.51.100.18",
    version: "1.8.2",
    os: "linux",
    arch: "arm64",
    capabilities: ["docker", "qr-relay"],
    tags: ["production", "emea"],
    deployment_count: 2,
    last_seen_at: iso(0.8),
  },
  {
    id: 3,
    name: "lab-03",
    status: "offline",
    agent_id: "agent-lab-03",
    hostname: "lab-03",
    ip_address: "192.0.2.27",
    version: "1.7.9",
    os: "linux",
    arch: "amd64",
    capabilities: ["docker"],
    tags: ["lab"],
    deployment_count: 0,
    last_seen_at: iso(180),
  },
];

const pageNames = [
  ["q4-awareness-login", "Quarterly awareness landing page", "https://example.com/q4-login"],
  ["finance-login", "Finance portal login page", "https://example.com/finance"],
  ["helpdesk-portal", "IT helpdesk login page", "https://example.com/helpdesk"],
];

const demoPages = pageNames.map(([name, description, url], index) => ({
  id: index + 1,
  name,
  description,
  url,
  created_by: 1,
  created_at: iso(2400 - index * 300),
  updated_at: iso(30 + index * 12),
}));

function demoPageHtml(name: string) {
  return `<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>${name}</title>
<style>
  *{box-sizing:border-box}body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;background:linear-gradient(135deg,#e0e7ff,#f8fafc 55%,#dbeafe);font-family:ui-sans-serif,system-ui,sans-serif;color:#0f172a}
  .card{width:min(360px,90vw);background:#fff;border:1px solid #e2e8f0;border-radius:12px;padding:30px;box-shadow:0 18px 50px rgba(15,23,42,.12)}
  .mark{width:42px;height:42px;border-radius:10px;background:#0f172a;color:#fff;display:flex;align-items:center;justify-content:center;font-weight:700;margin-bottom:18px}
  h1{font-size:20px;margin:0 0 6px}p{font-size:12px;color:#64748b;margin:0 0 22px}
  label{display:block;font-size:11px;font-weight:600;margin:12px 0 5px}input{width:100%;height:38px;border:1px solid #cbd5e1;border-radius:6px;padding:0 10px;font-size:13px;background:#f8fafc}
  button{width:100%;height:38px;margin-top:18px;border:0;border-radius:6px;background:#0f172a;color:#fff;font-weight:600}
  .foot{margin-top:16px;text-align:center;color:#94a3b8;font-size:10px}
</style></head>
<body><main class="card"><div class="mark">S</div><h1>${name}</h1><p>Authorized simulation preview · demo data</p>
<label>Username</label><input placeholder="you@example.com" autocomplete="off">
<label>Password</label><input type="password" placeholder="••••••••" autocomplete="off">
<button type="button">Sign in</button><div class="foot">This is a static demo page.</div></main></body></html>`;
}

const hostSamples = Array.from({ length: 48 }, (_, index) => {
  const minutes = (47 - index) * 30;
  return {
    id: index + 1,
    collected_at: iso(minutes),
    cpu_usage_percent: Number((9 + 13 * Math.sin(index / 5) + (index % 7)).toFixed(1)),
    memory_usage_percent: Number((42 + 7 * Math.sin(index / 7) + (index % 4)).toFixed(1)),
    disk_usage_percent: 18.6,
    memory_total_mb: 8192,
    memory_used_mb: 3900,
    disk_total_gb: 80,
    disk_used_gb: 14.9,
  };
});

const dashboard = {
  projectCount: 3,
  runningProjectCount: 2,
  mailCampaignCount: 4,
  messageCount: 27,
  agentCount: 3,
  onlineAgentCount: 2,
  offlineAgentCount: 1,
  activeDeploymentCount: 4,
  hostMetrics: {
    hostname: "spearhook-demo",
    os: "linux",
    arch: "x86_64",
    cpu_cores: 4,
    cpu_usage_percent: 18.4,
    memory_total_mb: 8192,
    memory_used_mb: 3912,
    memory_usage_percent: 47.8,
    disk_total_gb: 80,
    disk_used_gb: 14.9,
    disk_usage_percent: 18.6,
    uptime_seconds: 342_840,
    collected_at: iso(0.2),
  },
  hostMetricSamples: hostSamples,
};

const credentials = [
  {
    id: 101,
    created_at: iso(2),
    ip_address: "203.0.113.24",
    ip_location: "Singapore",
    username: "f.ops@example.com",
    password: "demo-password-1",
    captchavalue: "",
  },
  {
    id: 102,
    created_at: iso(6),
    ip_address: "198.51.100.17",
    ip_location: "Frankfurt, DE",
    username: "m.chen@example.com",
    password: "demo-password-2",
    captchavalue: "482913",
  },
  {
    id: 103,
    created_at: iso(19),
    ip_address: "203.0.113.88",
    ip_location: "Sydney, AU",
    username: "a.singh@example.com",
    password: "demo-password-3",
    captchavalue: "",
  },
];

const deployments = [
  {
    id: 11,
    agent_id: 1,
    agent: { name: "edge-sgp-01" },
    actual_status: "running",
    deployed_revision: 4,
    runtime_url: "http://edge-sgp.example.com:20001",
    runtime_port: 20001,
  },
  {
    id: 12,
    agent_id: 2,
    agent: { name: "edge-fra-02" },
    actual_status: "running",
    deployed_revision: 4,
    runtime_url: "http://edge-fra.example.com:20002",
    runtime_port: 20002,
  },
];

const chatSession = {
  id: 1,
  title: "Clone login page and deploy",
  created_at: iso(18),
  updated_at: iso(2),
  pending_confirm: false,
};

const chatMessages = {
  session: chatSession,
  messages: [
    {
      id: 1,
      role: "user",
      content: "Clone https://example.com/login into the page library and deploy it to an online agent.",
      created_at: iso(18),
    },
    {
      id: 2,
      role: "assistant",
      content: "I’ll resolve an available agent, mirror the page, and prepare the project.",
      created_at: iso(17),
    },
    {
      id: 3,
      role: "tool",
      tool_name: "list_agents",
      tool_call_id: "demo-tool-1",
      status: "ok",
      content: JSON.stringify({ summary: "3 agents found; edge-sgp-01 selected." }),
      created_at: iso(17),
    },
    {
      id: 4,
      role: "tool",
      tool_name: "clone_page",
      tool_call_id: "demo-tool-2",
      status: "ok",
      content: JSON.stringify({
        summary: "Page #4 “example-login” created; AI rewrite completed.",
        links: [{ label: "Open page library", href: "/workbench/page-builder" }],
      }),
      created_at: iso(16),
    },
    {
      id: 5,
      role: "assistant",
      content:
        "The page is mirrored and the deployment plan is ready. In a live console, this write step would require operator confirmation before creating the project.",
      created_at: iso(15),
    },
  ],
};

const mailCampaigns = [
  {
    id: 1,
    subject: "Q4 benefits enrollment",
    status: "sent",
    is_html: true,
    track_opens: true,
    sent_count: 42,
    open_count: 31,
    click_count: 12,
    failed_count: 1,
    recipient_count: 42,
    template_id: 1,
    scheduled_at: null,
    created_at: iso(420),
  },
  {
    id: 2,
    subject: "Password expiration notice",
    status: "scheduled",
    is_html: true,
    track_opens: true,
    sent_count: 0,
    open_count: 0,
    click_count: 0,
    failed_count: 0,
    recipient_count: 18,
    template_id: 1,
    scheduled_at: new Date(now + 86_400_000).toISOString(),
    created_at: iso(60),
  },
];

const mailTemplates = [
  {
    id: 1,
    name: "Enrollment notice",
    subject: "Action required: review your benefits",
    body: "<p>Demo template content.</p>",
    is_html: true,
    updated_at: iso(300),
  },
  {
    id: 2,
    name: "Password reminder",
    subject: "Your password expires soon",
    body: "<p>Demo template content.</p>",
    is_html: true,
    updated_at: iso(180),
  },
];

const smtpServices = [
  {
    id: 1,
    name: "Primary relay",
    host: "smtp.example.com",
    port: 587,
    from_email: "security@example.com",
    from_name: "Security Team",
    use_tls: true,
    use_ssl: false,
    enabled: true,
    description: "Demo SMTP profile",
  },
];

const robots = [
  {
    id: 1,
    name: "Security Operations",
    type: "slack",
    webhook_url: "https://hooks.example.com/services/demo",
    enabled: true,
  },
];

const qrRelays = [
  {
    id: 1,
    name: "Demo QR relay",
    slug: "demo-qr-relay",
    enabled: true,
    content_type: "image/png",
    image_bytes: 12_480,
    payload: "https://example.com/login",
    has_image: true,
    public_url: "https://example.com/q/demo-qr-relay.png",
    public_path: "/q/demo-qr-relay.png",
    placeholder_url: "https://example.com/q/demo-qr-relay.png",
    placeholder_img: '<img src="https://example.com/q/demo-qr-relay.png" alt="qr" width="240" height="240" />',
    last_upload_at: iso(8),
    last_seen_at: iso(1),
    upload_count: 27,
    health: "healthy",
    created_by: 1,
    created_at: iso(600),
    updated_at: iso(1),
  },
];

const infoJobs = [
  {
    id: 1,
    target: "example.com",
    notes: "Public security contacts",
    include_x_search: false,
    status: "succeeded",
    email_count: 4,
    phone_count: 2,
    finding_count: 6,
    progress_stage: "done",
    progress_percent: 100,
    progress_message: "Completed",
    summary: "Public contacts collected from official pages.",
    error: "",
    created_at: iso(120),
    updated_at: iso(115),
    started_at: iso(119),
    finished_at: iso(115),
  },
];

const infoFindings = [
  { id: 1, kind: "email", value: "security@example.com", label: "Security team", source_url: "https://example.com/contact", snippet: "security@example.com" },
  { id: 2, kind: "email", value: "press@example.com", label: "Press", source_url: "https://example.com/press", snippet: "press@example.com" },
  { id: 3, kind: "phone", value: "+1-555-010-2048", label: "Main office", source_url: "https://example.com/contact", snippet: "+1-555-010-2048" },
];

const auditLogs = {
  items: [
    {
      id: 506,
      actor_id: 1,
      actor_name: "demo",
      action: "create_project",
      resource_type: "project",
      resource_id: "2",
      method: "POST",
      path: "/api/projects/",
      status_code: 201,
      success: true,
      summary: "Created project finance-login",
      detail: "",
      client_ip: "203.0.113.42",
      user_agent: "demo",
      created_at: iso(8),
    },
    {
      id: 505,
      actor_id: 1,
      actor_name: "demo",
      action: "view_credentials",
      resource_type: "credential",
      resource_id: "101",
      method: "GET",
      path: "/api/projects/1/credentials/",
      status_code: 200,
      success: true,
      summary: "Viewed credential #101",
      detail: "",
      client_ip: "203.0.113.42",
      user_agent: "demo",
      created_at: iso(21),
    },
    {
      id: 504,
      actor_id: 1,
      actor_name: "demo",
      action: "login_success",
      resource_type: "auth",
      resource_id: "",
      method: "POST",
      path: "/api/auth/login/",
      status_code: 200,
      success: true,
      summary: "demo login success",
      detail: "",
      client_ip: "203.0.113.42",
      user_agent: "demo",
      created_at: iso(60),
    },
  ],
  total: 3,
  page: 1,
  page_size: 50,
};

const users = [
  { id: 1, username: "demo", role: "admin", created_at: iso(10_000), updated_at: iso(10) },
  { id: 2, username: "analyst", role: "operator", created_at: iso(8000), updated_at: iso(30) },
];

const aiSettings = {
  profiles: [
    {
      id: "demo-ai",
      name: "Demo AI profile",
      enabled: true,
      api_format: "openai",
      base_url: "https://api.example.com/v1",
      api_key: "demo-••••••••",
      api_key_set: true,
      model: "gpt-4.1-mini",
      timeout_sec: 120,
      prompt_tokens: 121_324,
      completion_tokens: 2_979,
      total_tokens: 124_303,
      request_count: 27,
      last_used_at: iso(2),
    },
  ],
};

const searchSettings = {
  profiles: [
    {
      id: "demo-search",
      name: "Demo search",
      enabled: true,
      provider: "brave",
      base_url: "https://api.search.brave.com/res/v1",
      api_key: "••••••••",
      api_key_set: true,
      count: 10,
    },
  ],
  providers: [
    { id: "auto", label: "AI Profile (native web_search)", default_base_url: "", requires_key: false, native: true },
    { id: "brave", label: "Brave Search", default_base_url: "https://api.search.brave.com/res/v1", requires_key: true },
    { id: "tavily", label: "Tavily", default_base_url: "https://api.tavily.com", requires_key: true },
    { id: "deepseek", label: "DeepSeek Search", default_base_url: "https://api.deepseek.com", requires_key: true },
  ],
};

const mailTracking = {
  public_base_url: "https://console.example.com",
  enabled: true,
  secret: "demo-••••••••",
  secret_set: true,
  allow_redirect_hosts: [],
};

const ipBlacklist = [
  { id: 1, ip_address: "203.0.113.88", reason: "Repeated automated submissions", enabled: true, created_at: iso(34) },
  { id: 2, ip_address: "198.51.100.77", reason: "Credential spraying", enabled: true, created_at: iso(180) },
];

function response(config: InternalAxiosRequestConfig, data: unknown, status = 200): AxiosResponse {
  return {
    data,
    status,
    statusText: status >= 200 && status < 300 ? "OK" : "Error",
    headers: {},
    config,
    request: {},
  };
}

function pathOf(config: InternalAxiosRequestConfig) {
  return String(config.url || "").split("?")[0];
}

function isGet(config: InternalAxiosRequestConfig) {
  return String(config.method || "get").toLowerCase() === "get";
}

function projectById(id: string) {
  const project = demoProjects.find((item) => String(item.id) === id) || demoProjects[0];
  return {
    ...project,
    container_code: "from flask import Flask\napp = Flask(__name__)\n\n@app.route('/')\ndef index():\n    return 'Demo landing page'\n",
    html_file_path: "/tmp/demo/index.html",
  };
}

const adapters: Array<{
  pattern: RegExp;
  method?: string;
  data: (config: InternalAxiosRequestConfig) => Json;
}> = [
  { pattern: /^\/api\/auth\/me\/?$/, data: () => DEMO_USER },
  { pattern: /^\/api\/dashboard\/?$/, data: () => dashboard },
  { pattern: /^\/api\/projects\/?$/, data: () => demoProjects },
  { pattern: /^\/api\/projects\/([^/]+)\/?$/, data: (c) => projectById(String(c.url).match(/projects\/([^/]+)/)?.[1] || "1") },
  { pattern: /^\/api\/projects\/[^/]+\/deployments\/?$/, data: () => deployments },
  { pattern: /^\/api\/projects\/[^/]+\/credentials\/?$/, data: () => credentials },
  { pattern: /^\/api\/projects\/[^/]+\/logs\/?$/, data: () => [
    { id: 1, logged_at: iso(3), agent_id: 1, level: "info", message: "Request served: GET /portal", deployment_id: 11 },
    { id: 2, logged_at: iso(2), agent_id: 1, level: "info", message: "Credential submission forwarded to control plane", deployment_id: 11 },
    { id: 3, logged_at: iso(1), agent_id: 2, level: "warn", message: "Reverse proxy health check took 420ms", deployment_id: 12 },
  ] },
  { pattern: /^\/api\/projects\/[^/]+\/container_log\/?$/, data: () => ({
    log: "[demo] Flask application starting\n[demo] Listening on 127.0.0.1:8090\n[demo] Ready\n[demo] GET / 200\n[demo] POST /api/submit 201\n",
  }) },
  { pattern: /^\/api\/projects\/[^/]+\/build_status\/?$/, data: () => ({ status: "success", percent: 100, message: "Demo build complete" }) },
  { pattern: /^\/api\/agents\/?$/, data: () => demoAgents },
  { pattern: /^\/api\/agents\/([^/]+)\/?$/, data: () => demoAgents[0] },
  { pattern: /^\/api\/phishing-pages\/?$/, data: () => demoPages },
  { pattern: /^\/api\/phishing-pages\/([^/]+)\/?$/, data: (c) => {
    const id = Number(String(c.url).match(/phishing-pages\/([^/]+)/)?.[1] || 1);
    const page = demoPages.find((item) => item.id === id) || demoPages[0];
    return { ...page, html: demoPageHtml(page.name) };
  } },
  { pattern: /^\/api\/chat\/sessions\/?$/, method: "get", data: () => [chatSession, { ...chatSession, id: 2, title: "Collect public contacts", pending_confirm: false }] },
  { pattern: /^\/api\/chat\/sessions\/([^/]+)\/messages\/?$/, data: () => chatMessages },
  { pattern: /^\/api\/mail-campaigns\/?$/, data: () => mailCampaigns },
  { pattern: /^\/api\/mail-campaigns\/([^/]+)\/?$/, data: () => mailCampaigns[0] },
  { pattern: /^\/api\/mail-campaigns\/[^/]+\/recipients\/?$/, data: () => [] },
  { pattern: /^\/api\/mail-campaigns\/[^/]+\/events\/?$/, data: () => [] },
  { pattern: /^\/api\/mail-templates\/?$/, data: () => mailTemplates },
  { pattern: /^\/api\/mail-templates\/([^/]+)\/?$/, data: () => mailTemplates[0] },
  { pattern: /^\/api\/smtp-services\/?$/, data: () => smtpServices },
  { pattern: /^\/api\/smtp-services\/([^/]+)\/?$/, data: () => smtpServices[0] },
  { pattern: /^\/api\/robots\/?$/, data: () => robots },
  { pattern: /^\/api\/qr-relays\/?$/, data: () => qrRelays },
  { pattern: /^\/api\/qr-relays\/([^/]+)\/?$/, data: () => qrRelays[0] },
  { pattern: /^\/api\/qr-relays\/[^/]+\/frames\/?$/, data: () => [
    { id: 1, uploaded_at: iso(12), is_current: true, image_url: "https://example.com/frame-1.png" },
    { id: 2, uploaded_at: iso(40), is_current: false, image_url: "https://example.com/frame-2.png" },
  ] },
  { pattern: /^\/api\/info-gather-jobs\/?$/, data: () => infoJobs },
  { pattern: /^\/api\/info-gather-jobs\/([^/]+)\/?$/, data: () => infoJobs[0] },
  { pattern: /^\/api\/info-gather-jobs\/[^/]+\/findings\/?$/, data: () => infoFindings },
  { pattern: /^\/api\/audit-logs\/?$/, data: () => auditLogs },
  { pattern: /^\/api\/audit-logs\/actions\/?$/, data: () => ["login_success", "create_project", "view_credentials"] },
  { pattern: /^\/api\/users\/?$/, data: () => users },
  { pattern: /^\/api\/ai-settings\/?$/, data: () => aiSettings },
  { pattern: /^\/api\/search-settings\/?$/, data: () => searchSettings },
  { pattern: /^\/api\/mail-tracking-settings\/?$/, data: () => mailTracking },
  { pattern: /^\/api\/ip-blacklist\/?$/, data: () => ipBlacklist },
];

function fallback(config: InternalAxiosRequestConfig): Json {
  const path = pathOf(config);
  if (isGet(config)) {
    if (path.includes("test") || path.includes("status")) return { ok: true, message: "Demo mode" };
    return [];
  }
  return { ok: true, message: "Demo mode: action simulated." };
}

export const demoAdapter: AxiosAdapter = async (config) => {
  await new Promise((resolve) => setTimeout(resolve, 70));
  if (String(config.responseType || "") === "blob") {
    return response(config, new Blob(["demo"], { type: "application/zip" }));
  }
  if (!isGet(config)) {
    if (pathOf(config) === "/api/auth/logout/") return response(config, { ok: true });
    if (pathOf(config) === "/api/chat/sessions/") return response(config, { id: 1 });
    if (pathOf(config) === "/api/ai-settings/test/" || pathOf(config) === "/api/search-settings/test/") {
      return response(config, {
        ok: true,
        chat: { ok: true, latency_ms: 28, status: 200, endpoint: "demo" },
        web_search: { ok: true, latency_ms: 35, status: 200, mode: "demo" },
      });
    }
    return response(config, fallback(config));
  }
  const match = adapters.find((adapter) => adapter.pattern.test(pathOf(config)));
  return response(config, match ? match.data(config) : fallback(config));
};

export async function streamDemoChat(
  onEvent: (event: { event: string; data: Record<string, unknown> }) => void,
  signal?: AbortSignal,
) {
  const pieces = [
    "This is a read-only demo. ",
    "In a live console, I would resolve an online agent, mirror the page, ",
    "create the project, and pause for confirmation before deploying.",
  ];
  onEvent({ event: "thinking", data: { phase: "plan", message: "Preparing demo response" } });
  for (const piece of pieces) {
    if (signal?.aborted) return;
    onEvent({ event: "token", data: { content: piece } });
    await new Promise((resolve) => setTimeout(resolve, 80));
  }
  onEvent({ event: "done", data: {} });
}

export async function streamDemoEvents(
  _onEvent: (event: { event: string; data: Record<string, unknown> }) => void,
  signal?: AbortSignal,
) {
  if (signal?.aborted) return;
  await new Promise((resolve) => setTimeout(resolve, 150));
}
