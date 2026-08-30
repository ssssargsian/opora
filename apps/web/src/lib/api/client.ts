export type APIErrorBody = { error?: { code?: string; message?: string; requestId?: string } };

export class APIError extends Error {
  constructor(public readonly status: number, public readonly code: string, message: string) {
    super(message);
  }
}

function csrfToken(): string | undefined {
  if (typeof document === "undefined") return undefined;
  return document.cookie.split("; ").find((part) => part.startsWith("opora_csrf="))?.split("=").slice(1).join("=");
}

export async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const method = (init.method ?? "GET").toUpperCase();
  const headers = new Headers(init.headers);
  if (init.body && !(init.body instanceof FormData) && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");
  if (!["GET", "HEAD", "OPTIONS"].includes(method)) {
    const token = csrfToken();
    if (token) headers.set("X-CSRF-Token", decodeURIComponent(token));
  }
  let response: Response;
  try {
    response = await fetch(path, { ...init, headers, credentials: "include", cache: "no-store" });
  } catch {
    throw new APIError(0, "network_error", "Не удалось связаться с сервером");
  }
  if (!response.ok) {
    const body = await response.json().catch(() => ({})) as APIErrorBody;
    throw new APIError(response.status, body.error?.code ?? "request_failed", body.error?.message ?? "Request failed");
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

export type CurrentUser = {
  id: string;
  email: string;
  displayName: string;
  lastName: string;
  firstName: string;
  middleName: string | null;
  organization: { id: string; name: string };
  permissions: string[];
};

export const authAPI = {
  me: () => apiFetch<CurrentUser>("/api/v1/me"),
  login: (email: string, password: string) => apiFetch<CurrentUser>("/api/v1/auth/login", {
    method: "POST", body: JSON.stringify({ email, password }),
  }),
  logout: () => apiFetch<void>("/api/v1/auth/logout", { method: "POST" }),
  updateProfile: (input: { lastName: string; firstName: string; middleName?: string; email: string }) =>
    apiFetch("/api/v1/me", { method: "PATCH", body: JSON.stringify({ ...input, middleName: input.middleName || null }) }),
  changePassword: (input: { currentPassword: string; newPassword: string }) =>
    apiFetch<void>("/api/v1/me/change-password", { method: "POST", body: JSON.stringify(input) }),
  acceptInvitation: (token: string, password: string) =>
    apiFetch<{ email: string }>("/api/v1/auth/invitations/accept", { method: "POST", body: JSON.stringify({ token, password }) }),
};

export const organizationAPI = {
  update: (name: string) => apiFetch<{ id: string; name: string; updatedAt: string }>("/api/v1/organization", {
    method: "PATCH", body: JSON.stringify({ name }),
  }),
};
