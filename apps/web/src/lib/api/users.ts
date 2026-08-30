import { apiFetch } from "./client";

export type OrganizationRole = {
  id: string;
  key: string;
  name: string;
  isSystem: boolean;
};

export type OrganizationUser = {
  id: string;
  roleId: string;
  displayName: string;
  lastName: string;
  firstName: string;
  middleName?: string | null;
  email: string;
  roleKey: string;
  roleName: string;
  status: "active" | "invited" | "blocked";
  createdAt: string;
  invitationCreatedAt?: string | null;
  invitationAcceptedAt?: string | null;
  invitationDelivery?: "sent" | "failed";
};

export const usersAPI = {
  list: async (): Promise<OrganizationUser[]> =>
    (await apiFetch<{ items: OrganizationUser[] }>("/api/v1/users")).items,
  roles: async (): Promise<OrganizationRole[]> =>
    (await apiFetch<{ items: OrganizationRole[] }>("/api/v1/roles")).items,
  create: (input: { lastName: string; firstName: string; middleName?: string; email: string; roleKey: string }) =>
    apiFetch<OrganizationUser>("/api/v1/users", { method: "POST", body: JSON.stringify(input) }),
  update: (userId: string, input: { lastName: string; firstName: string; middleName?: string; email: string; roleKey: string }) =>
    apiFetch<OrganizationUser>(`/api/v1/users/${encodeURIComponent(userId)}`, { method: "PATCH", body: JSON.stringify(input) }),
  resendInvitation: (userId: string) => apiFetch<OrganizationUser>(`/api/v1/users/${encodeURIComponent(userId)}/invitation`, { method: "POST" }),
};
