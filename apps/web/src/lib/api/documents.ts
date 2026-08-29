import { apiFetch } from "./client";
import type { DocumentKind, DocumentVersion, StudentDocument } from "@/lib/data/types";

type APIVersion = { id: string; versionNumber: number; originalFilename: string; mimeType: string; size: number; sha256: string; changedBy: string; createdAt: string };
type APIDocument = { id: string; studentId: string; title: string; documentType: string | null; confidentialityLevel: "standard" | "restricted"; currentVersion: APIVersion; createdAt: string; updatedAt: string };
const dateTime = (value: string) => new Intl.DateTimeFormat("ru-RU", { dateStyle: "medium", timeStyle: "short" }).format(new Date(value));
const kind = (mimeType: string): DocumentKind => mimeType === "application/pdf" ? "pdf" : "docx";
const mapVersion = (version: APIVersion): DocumentVersion => ({ ...version, changedAt: dateTime(version.createdAt) });
const mapDocument = (document: APIDocument): StudentDocument => ({
  id: document.id, studentId: document.studentId, title: document.title, documentType: document.documentType,
  confidentialityLevel: document.confidentialityLevel, kind: kind(document.currentVersion.mimeType),
  currentVersion: mapVersion(document.currentVersion), updatedAt: dateTime(document.updatedAt),
});

export const documentsAPI = {
  async list(studentId: string): Promise<StudentDocument[]> {
    const response = await apiFetch<{ items: APIDocument[] }>(`/api/v1/students/${encodeURIComponent(studentId)}/documents`);
    return response.items.map(mapDocument);
  },
  async versions(documentId: string): Promise<DocumentVersion[]> {
    const response = await apiFetch<{ items: APIVersion[] }>(`/api/v1/documents/${encodeURIComponent(documentId)}/versions`);
    return response.items.map(mapVersion);
  },
  upload(studentId: string, form: FormData): Promise<StudentDocument> {
    return apiFetch<APIDocument>(`/api/v1/students/${encodeURIComponent(studentId)}/documents`, { method: "POST", body: form }).then(mapDocument);
  },
  editor(documentId: string): Promise<{ documentServerUrl: string; config: Record<string, unknown> }> {
    return apiFetch(`/api/v1/documents/${encodeURIComponent(documentId)}/editor`);
  },
  downloadURL(documentId: string, versionId?: string): string {
    return versionId ? `/api/v1/documents/${encodeURIComponent(documentId)}/versions/${encodeURIComponent(versionId)}/download`
      : `/api/v1/documents/${encodeURIComponent(documentId)}/download`;
  },
  previewURL(documentId: string): string {
    return `/api/v1/documents/${encodeURIComponent(documentId)}/preview`;
  },
};

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} Б`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} КБ`;
  return `${(bytes / 1024 / 1024).toFixed(1)} МБ`;
}
