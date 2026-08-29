import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { Student } from "@/lib/data/types";
import { StudentWorkspace } from "./student-workspace";

const student: Student = {
  id: "student-1", fullName: "Иванов Иван Иванович", className: "7А", birthDate: "14 марта 2013", documentCount: 1, updatedAt: "29.08.2026",
  access: [],
};

const { mockDocument } = vi.hoisted(() => ({ mockDocument: { id: "doc-1", studentId: "student-1", title: "Заключение", documentType: null, confidentialityLevel: "standard" as const, kind: "docx" as const, updatedAt: "29.08.2026", currentVersion: { id: "v2", versionNumber: 2, originalFilename: "file.docx", mimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", size: 1500, sha256: "a".repeat(64), changedAt: "29.08.2026", changedBy: "Анна Петрова" } } }));

vi.mock("@/features/auth/auth-boundary", () => ({ useCurrentUser: () => ({ permissions: ["documents.upload", "documents.download", "documents.edit", "access.view", "access.manage"] }) }));
vi.mock("@/lib/api/documents", () => ({
  documentsAPI: { list: vi.fn().mockResolvedValue([mockDocument]), versions: vi.fn().mockResolvedValue([mockDocument.currentVersion]), downloadURL: () => "/download", previewURL: () => "/preview" },
  formatBytes: () => "1.5 КБ",
}));
vi.mock("@/lib/api/access", () => ({
  accessAPI: {
    list: vi.fn().mockResolvedValue([{ userId: "user-1", displayName: "Анна Петрова", email: "anna@test.local", roleName: "Психолог", grants: ["view"] }]),
    set: vi.fn(),
    revoke: vi.fn(),
  },
}));

function renderWorkspace() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}><StudentWorkspace student={student} /></QueryClientProvider>);
}

describe("StudentWorkspace", () => {
  it("loads real documents, opens history and switches to access grants", async () => {
    renderWorkspace();
    expect(await screen.findByText("Заключение")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "История" }));
    expect(await screen.findByRole("dialog", { name: "Заключение" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Закрыть" }));
    fireEvent.click(screen.getByRole("tab", { name: /Доступ/ }));
    expect(screen.getByText(/Психолог · anna@test\.local/)).toBeInTheDocument();
    expect(screen.getByText("Просмотр")).toBeInTheDocument();
  });
});
