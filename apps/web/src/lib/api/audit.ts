import { apiFetch } from "./client";

export type AuditEvent = {
  id: string;
  actorName: string;
  action: string;
  resourceType: string;
  resourceId: string | null;
  createdAt: string;
  metadata: Record<string, unknown>;
};

export const auditAPI = {
  list: async (): Promise<AuditEvent[]> =>
    (await apiFetch<{ items: AuditEvent[] }>("/api/v1/audit")).items,
};
