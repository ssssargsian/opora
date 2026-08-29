import { apiFetch } from "./client";

export type StudentGrant = "view" | "upload" | "download" | "edit";

export type StudentAssignment = {
  userId: string;
  displayName: string;
  email: string;
  roleKey: string;
  roleName: string;
  grants: StudentGrant[];
};

export const accessAPI = {
  list: async (studentId: string): Promise<StudentAssignment[]> =>
    (await apiFetch<{ items: StudentAssignment[] }>(`/api/v1/students/${encodeURIComponent(studentId)}/access`)).items,
  set: (studentId: string, userId: string, grants: StudentGrant[]) =>
    apiFetch<void>(`/api/v1/students/${encodeURIComponent(studentId)}/access`, {
      method: "POST", body: JSON.stringify({ userId, grants }),
    }),
  revoke: (studentId: string, userId: string) =>
    apiFetch<void>(`/api/v1/students/${encodeURIComponent(studentId)}/access/${encodeURIComponent(userId)}`, {
      method: "DELETE",
    }),
};
