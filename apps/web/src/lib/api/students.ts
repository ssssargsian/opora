import { apiFetch } from "./client";
import type { Student, StudentSummary } from "@/lib/data/types";

type APIStudent = {
  id: string; lastName: string; firstName: string; middleName: string | null; birthDate: string | null;
  className: string | null; documentCount: number; createdAt: string; updatedAt: string;
};

const fullName = (student: APIStudent) => [student.lastName, student.firstName, student.middleName].filter(Boolean).join(" ");
const dateTime = (value: string) => new Intl.DateTimeFormat("ru-RU", { dateStyle: "medium", timeStyle: "short" }).format(new Date(value));
const mapSummary = (student: APIStudent): StudentSummary => ({
  id: student.id, fullName: fullName(student), className: student.className ?? "—",
  documentCount: student.documentCount, updatedAt: dateTime(student.updatedAt),
});

export const studentsAPI = {
  async list(): Promise<StudentSummary[]> {
    const response = await apiFetch<{ items: APIStudent[] }>("/api/v1/students");
    return response.items.map(mapSummary);
  },
  async get(id: string): Promise<Student> {
    const student = await apiFetch<APIStudent>(`/api/v1/students/${encodeURIComponent(id)}`);
    return { ...mapSummary(student), lastName: student.lastName, firstName: student.firstName,
      middleName: student.middleName ?? "", birthDateValue: student.birthDate ?? "", birthDate: student.birthDate
        ? new Intl.DateTimeFormat("ru-RU", { dateStyle: "long" }).format(new Date(student.birthDate)) : "Не указана", access: [] };
  },
  async create(input: { lastName: string; firstName: string; middleName?: string; birthDate?: string; className?: string }): Promise<StudentSummary> {
    const student = await apiFetch<APIStudent>("/api/v1/students", {
      method: "POST",
      body: JSON.stringify({
        lastName: input.lastName,
        firstName: input.firstName,
        middleName: input.middleName || null,
        birthDate: input.birthDate || null,
        className: input.className || null,
      }),
    });
    return mapSummary(student);
  },
  async update(id: string, input: { lastName: string; firstName: string; middleName?: string; birthDate?: string; className?: string }): Promise<Student> {
    await apiFetch<APIStudent>(`/api/v1/students/${encodeURIComponent(id)}`, {
      method: "PATCH",
      body: JSON.stringify({ ...input, middleName: input.middleName || null, birthDate: input.birthDate || null, className: input.className || null }),
    });
    return studentsAPI.get(id);
  },
};
