import { apiClient } from "./client";
import { DEMO_MODE } from "@/demo/config";
import { streamDemoChat, streamDemoEvents } from "@/demo/mock";

export const api = {
  login(data: { username: string; password: string }) {
    return apiClient.post("/api/auth/login/", data);
  },
  logout() {
    return apiClient.post("/api/auth/logout/");
  },
  getCurrentUser() {
    return apiClient.get("/api/auth/me/");
  },
  getUsers() {
    return apiClient.get("/api/users/");
  },
  createUser(data: { username: string; password: string; role?: string }) {
    return apiClient.post("/api/users/", data);
  },
  updateUser(id: number | string, data: { role?: string }) {
    return apiClient.put(`/api/users/${id}/`, data);
  },
  resetUserPassword(id: number | string) {
    return apiClient.post(`/api/users/${id}/reset-password/`);
  },
  deleteUser(id: number | string) {
    return apiClient.delete(`/api/users/${id}/`);
  },
  getDashboardStats() {
    return apiClient.get("/api/dashboard/");
  },
  getProjects() {
    return apiClient.get("/api/projects/");
  },
  createProject(data: FormData) {
    return apiClient.post("/api/projects/", data);
  },
  updateProject(id: number | string, data: FormData) {
    return apiClient.put(`/api/projects/${id}/`, data);
  },
  deleteProject(id: number | string) {
    return apiClient.delete(`/api/projects/${id}/`);
  },
  getProject(id: number | string) {
    return apiClient.get(`/api/projects/${id}/`);
  },
  startProject(id: number | string) {
    return apiClient.post(`/api/projects/${id}/start_container/`);
  },
  stopProject(id: number | string) {
    return apiClient.post(`/api/projects/${id}/stop_container/`);
  },
  startLocalProject(id: number | string) {
    return apiClient.post(`/api/projects/${id}/start_local/`);
  },
  stopLocalProject(id: number | string) {
    return apiClient.post(`/api/projects/${id}/stop_local/`);
  },
  buildProject(id: number | string, body: { force?: boolean } = {}) {
    return apiClient.post(`/api/projects/${id}/build/`, body);
  },
  getBuildStatus(id: number | string) {
    return apiClient.get(`/api/projects/${id}/build_status/`);
  },
  getProjectLogs(id: number | string) {
    return apiClient.get(`/api/projects/${id}/container_log/`);
  },
  getCentralLogs(id: number | string, limit = 500) {
    return apiClient.get(`/api/projects/${id}/logs/`, { params: { limit } });
  },
  getDeployments(id: number | string) {
    return apiClient.get(`/api/projects/${id}/deployments/`);
  },
  controlDeployment(id: number | string, deploymentId: number | string, action: string) {
    return apiClient.post(`/api/projects/${id}/deployments/${deploymentId}/${action}/`);
  },
  getCredentials(id: number | string) {
    return apiClient.get(`/api/projects/${id}/credentials/`, {
      params: { _ts: Date.now() },
    });
  },
  getAgents() {
    return apiClient.get("/api/agents/");
  },
  getAgent(id: number | string) {
    return apiClient.get(`/api/agents/${id}/`);
  },
  getRobots() {
    return apiClient.get("/api/robots/");
  },
  createRobot(data: unknown) {
    return apiClient.post("/api/robots/", data);
  },
  updateRobot(id: number | string, data: unknown) {
    return apiClient.put(`/api/robots/${id}/`, data);
  },
  deleteRobot(id: number | string) {
    return apiClient.delete(`/api/robots/${id}/`);
  },
  onlineRobot(id: number | string) {
    return apiClient.post(`/api/robots/${id}/online/`);
  },
  offlineRobot(id: number | string) {
    return apiClient.post(`/api/robots/${id}/offline/`);
  },
  testRobot(id: number | string, data: unknown) {
    return apiClient.post(`/api/robots/${id}/test/`, data);
  },
  getIPBlacklist() {
    return apiClient.get("/api/ip-blacklist/");
  },
  createIPBlacklistEntry(data: unknown) {
    return apiClient.post("/api/ip-blacklist/", data);
  },
  updateIPBlacklistEntry(id: number | string, data: unknown) {
    return apiClient.put(`/api/ip-blacklist/${id}/`, data);
  },
  deleteIPBlacklistEntry(id: number | string) {
    return apiClient.delete(`/api/ip-blacklist/${id}/`);
  },

  // SMTP services
  getSmtpServices() {
    return apiClient.get("/api/smtp-services/");
  },
  getSmtpService(id: number | string) {
    return apiClient.get(`/api/smtp-services/${id}/`);
  },
  createSmtpService(data: unknown) {
    return apiClient.post("/api/smtp-services/", data);
  },
  updateSmtpService(id: number | string, data: unknown) {
    return apiClient.put(`/api/smtp-services/${id}/`, data);
  },
  deleteSmtpService(id: number | string) {
    return apiClient.delete(`/api/smtp-services/${id}/`);
  },
  testSmtpService(id: number | string, data: unknown) {
    return apiClient.post(`/api/smtp-services/${id}/test/`, data);
  },
  sendSmtpServiceMail(id: number | string, data: unknown) {
    return apiClient.post(`/api/smtp-services/${id}/send/`, data);
  },

  // AI settings (admin)
  getAiSettings() {
    return apiClient.get("/api/ai-settings/");
  },
  updateAiSettings(data: unknown) {
    return apiClient.put("/api/ai-settings/", data);
  },
  testAiSettings(data: {
    id?: string;
    api_format?: string;
    base_url?: string;
    api_key?: string;
    model?: string;
    timeout_sec?: number;
  }) {
    return apiClient.post("/api/ai-settings/test/", data, { timeout: 120000 });
  },

  // Web search provider settings (admin)
  getSearchSettings() {
    return apiClient.get("/api/search-settings/");
  },
  updateSearchSettings(data: unknown) {
    return apiClient.put("/api/search-settings/", data);
  },
  testSearchSettings(data: {
    id?: string;
    provider?: string;
    base_url?: string;
    api_key?: string;
    count?: number;
  }) {
    return apiClient.post("/api/search-settings/test/", data, { timeout: 120000 });
  },

  // Chat console
  listChatSessions() {
    return apiClient.get("/api/chat/sessions/");
  },
  createChatSession() {
    return apiClient.post("/api/chat/sessions/");
  },
  getChatMessages(id: number | string) {
    return apiClient.get(`/api/chat/sessions/${id}/messages/`);
  },
  deleteChatSession(id: number | string) {
    return apiClient.delete(`/api/chat/sessions/${id}/`);
  },
  getMailTrackingSettings() {
    return apiClient.get("/api/mail-tracking-settings/");
  },
  updateMailTrackingSettings(data: unknown) {
    return apiClient.put("/api/mail-tracking-settings/", data);
  },

  // Phishing pages (page builder)
  getPhishingPages() {
    return apiClient.get("/api/phishing-pages/");
  },
  getPhishingPage(id: number | string) {
    return apiClient.get(`/api/phishing-pages/${id}/`);
  },
  // Prefer streamMirrorPhishingPage (SSE stages). Kept for non-UI callers.
  mirrorPhishingPage(data: {
    url: string;
    name?: string;
    description?: string;
    submit_url?: string;
    redirect_url?: string;
  }) {
    return apiClient.post("/api/phishing-pages/mirror/", data, {
      timeout: 180000,
      responseType: "text",
    });
  },
  upsertPhishingPage(data: {
    name: string;
    html: string;
    url?: string;
    description?: string;
  }) {
    return apiClient.post("/api/phishing-pages/upsert/", data, { timeout: 120000 });
  },
  deletePhishingPage(id: number | string) {
    return apiClient.delete(`/api/phishing-pages/${id}/`);
  },

  // Audit logs (read-only)
  getAuditLogs(params?: {
    page?: number;
    page_size?: number;
    from?: string;
    to?: string;
    action?: string;
    actor?: string;
    keyword?: string;
  }) {
    return apiClient.get("/api/audit-logs/", { params });
  },
  getAuditLogActions() {
    return apiClient.get("/api/audit-logs/actions/");
  },

  // Mail campaigns (workbench)
  getMailCampaigns() {
    return apiClient.get("/api/mail-campaigns/");
  },
  getMailCampaign(id: number | string) {
    return apiClient.get(`/api/mail-campaigns/${id}/`);
  },
  createMailCampaign(data: unknown) {
    return apiClient.post("/api/mail-campaigns/", data);
  },
  deleteMailCampaign(id: number | string) {
    return apiClient.delete(`/api/mail-campaigns/${id}/`);
  },
  getMailCampaignRecipients(id: number | string) {
    return apiClient.get(`/api/mail-campaigns/${id}/recipients/`);
  },
  getMailCampaignEvents(id: number | string) {
    return apiClient.get(`/api/mail-campaigns/${id}/events/`);
  },
  getMailTemplates() {
    return apiClient.get("/api/mail-templates/");
  },
  getMailTemplate(id: number | string) {
    return apiClient.get(`/api/mail-templates/${id}/`);
  },
  createMailTemplate(data: {
    name: string;
    subject: string;
    body: string;
    is_html?: boolean;
  }) {
    return apiClient.post("/api/mail-templates/", data);
  },
  updateMailTemplate(
    id: number | string,
    data: { name: string; subject: string; body: string; is_html?: boolean },
  ) {
    return apiClient.put(`/api/mail-templates/${id}/`, data);
  },
  deleteMailTemplate(id: number | string) {
    return apiClient.delete(`/api/mail-templates/${id}/`);
  },
  getInfoGatherJobs() {
    return apiClient.get("/api/info-gather-jobs/");
  },
  getInfoGatherJob(id: number | string) {
    return apiClient.get(`/api/info-gather-jobs/${id}/`);
  },
  getInfoGatherFindings(id: number | string) {
    return apiClient.get(`/api/info-gather-jobs/${id}/findings/`);
  },
  createInfoGatherJob(data: {
    target: string;
    notes?: string;
    include_x_search?: boolean;
  }) {
    return apiClient.post("/api/info-gather-jobs/", data, { timeout: 60000 });
  },
  retryInfoGatherJob(id: number | string) {
    return apiClient.post(`/api/info-gather-jobs/${id}/retry/`);
  },
  deleteInfoGatherJob(id: number | string) {
    return apiClient.delete(`/api/info-gather-jobs/${id}/`);
  },
  getQrRelays() {
    return apiClient.get("/api/qr-relays/");
  },
  getQrRelay(id: number | string) {
    return apiClient.get(`/api/qr-relays/${id}/`);
  },
  createQrRelay(data: { name: string; slug?: string }) {
    return apiClient.post("/api/qr-relays/", data);
  },
  rotateQrRelayToken(id: number | string) {
    return apiClient.post(`/api/qr-relays/${id}/rotate-token/`);
  },
  rotateQrRelaySlug(id: number | string) {
    return apiClient.post(`/api/qr-relays/${id}/rotate-slug/`);
  },
  setQrRelayEnabled(id: number | string, enabled: boolean) {
    return apiClient.post(`/api/qr-relays/${id}/enabled/`, { enabled });
  },
  deleteQrRelay(id: number | string) {
    return apiClient.delete(`/api/qr-relays/${id}/`);
  },
  getQrRelayFrames(id: number | string) {
    return apiClient.get(`/api/qr-relays/${id}/frames/`);
  },
  promoteQrRelayFrame(id: number | string, frameId: number | string) {
    return apiClient.post(`/api/qr-relays/${id}/frames/${frameId}/promote/`);
  },
  downloadQrRelayScript(relayId?: number | string) {
    const params = relayId != null && relayId !== "" ? { relay_id: relayId } : undefined;
    return apiClient.get("/api/qr-relays/script.zip", {
      params,
      responseType: "blob",
      timeout: 120000,
    });
  },
};


export default api;


export type ChatSSEEvent =
  | { event: "token"; data: { content?: string } }
  | {
      event: "thinking";
      data: { phase?: string; message?: string };
    }
  | { event: "tool_start"; data: { tool_call_id?: string; name?: string; arguments?: string } }
  | {
      event: "tool_result";
      data: {
        tool_call_id?: string;
        name?: string;
        ok?: boolean;
        summary?: string;
        error?: string;
        result?: unknown;
        links?: { label: string; href: string }[];
        ephemeral?: Record<string, unknown>;
      };
    }
  | {
      event: "confirm_required";
      data: {
        tool_call_id?: string;
        name?: string;
        arguments?: unknown;
        message?: string;
        risk?: string;
      };
    }
  | {
      event: "progress";
      data: {
        tool_call_id?: string;
        stage?: string;
        percent?: number;
        message?: string;
      };
    }
  | {
      event: "job_progress" | "job_done";
      data: {
        job_id?: number;
        status?: string;
        stage?: string;
        percent?: number;
        message?: string;
        email_count?: number;
        phone_count?: number;
        finding_count?: number;
        link?: string;
      };
    }
  | { event: "ready" | "ping"; data: Record<string, unknown> }
  | { event: "done"; data: Record<string, unknown> }
  | { event: "error"; data: { error?: string } };

async function readSSEBody(
  res: Response,
  onEvent: (ev: ChatSSEEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  if (!res.body) throw new Error("empty response body");
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  while (true) {
    if (signal?.aborted) {
      try {
        await reader.cancel();
      } catch {
        /* ignore */
      }
      throw new DOMException("Aborted", "AbortError");
    }
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const chunks = buffer.split("\n\n");
    buffer = chunks.pop() || "";
    for (const chunk of chunks) {
      const lines = chunk.split("\n");
      let eventName = "message";
      let data = "";
      for (const line of lines) {
        if (line.startsWith("event:")) eventName = line.slice(6).trim();
        if (line.startsWith("data:")) data += line.slice(5).trim();
      }
      if (!data) continue;
      try {
        onEvent({ event: eventName as ChatSSEEvent["event"], data: JSON.parse(data) } as ChatSSEEvent);
      } catch {
        /* ignore malformed */
      }
    }
  }
}

function sseError(res: Response): Promise<Error> | Error {
  return (async () => {
    let msg = `HTTP ${res.status}`;
    try {
      const j = await res.json();
      if (j?.error) msg = String(j.error);
    } catch {
      /* ignore */
    }
    return new Error(msg);
  })();
}

export async function streamChatRequest(
  url: string,
  body: unknown,
  onEvent: (ev: ChatSSEEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  if (DEMO_MODE) {
    await streamDemoChat(onEvent as (event: { event: string; data: Record<string, unknown> }) => void, signal);
    return;
  }
  const token =
    typeof window !== "undefined" ? localStorage.getItem("token") || "" : "";
  const res = await fetch(`${process.env.NEXT_PUBLIC_API_BASE || ""}${url}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify(body ?? {}),
    signal,
  });
  if (!res.ok) throw await sseError(res);
  await readSSEBody(res, onEvent, signal);
}

/** Long-lived GET stream for async job events in one session. */
export async function streamChatEvents(
  url: string,
  onEvent: (ev: ChatSSEEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  if (DEMO_MODE) {
    await streamDemoEvents(onEvent as (event: { event: string; data: Record<string, unknown> }) => void, signal);
    return;
  }
  const token =
    typeof window !== "undefined" ? localStorage.getItem("token") || "" : "";
  const res = await fetch(`${process.env.NEXT_PUBLIC_API_BASE || ""}${url}`, {
    method: "GET",
    headers: {
      Accept: "text/event-stream",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    signal,
  });
  if (!res.ok) throw await sseError(res);
  await readSSEBody(res, onEvent, signal);
}
