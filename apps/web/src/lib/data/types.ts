export type DocumentKind = "docx" | "pdf";

export type DocumentVersion = {
  id: string;
  versionNumber: number;
  originalFilename: string;
  mimeType: string;
  size: number;
  sha256: string;
  changedAt: string;
  changedBy: string;
};

export type StudentDocument = {
  id: string;
  studentId: string;
  title: string;
  documentType: string | null;
  confidentialityLevel: "standard" | "restricted";
  kind: DocumentKind;
  currentVersion: DocumentVersion;
  updatedAt: string;
};

export type StudentAccess = {
  id: string;
  name: string;
  specialty: string;
  grants: string[];
};

export type StudentSummary = {
  id: string;
  fullName: string;
  className: string;
  documentCount: number;
  updatedAt: string;
};

export type Student = StudentSummary & {
  birthDate: string;
  access: StudentAccess[];
};

export type Specialist = {
  id: string;
  name: string;
  email: string;
  role: string;
  status: "Активен" | "Приглашён" | "Заблокирован";
};

export type AuditEntry = {
  id: string;
  occurredAt: string;
  actor: string;
  action: string;
  object: string;
};

export type OrganizationProfile = {
  name: string;
  shortName: string;
  city: string;
  currentUser: { name: string; role: string };
};
