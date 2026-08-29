import type { StudentDataSource } from "../student-data-source";
import type { Student, StudentSummary } from "../types";

export class HTTPStudentDataSource implements StudentDataSource {
  constructor(private readonly baseURL: string) {}

  async list(): Promise<StudentSummary[]> {
    const response = await fetch(`${this.baseURL}/api/v1/students`, { credentials: "include", cache: "no-store" });
    if (!response.ok) throw new Error("Students API is unavailable");
    return response.json() as Promise<StudentSummary[]>;
  }

  async get(id: string): Promise<Student | null> {
    const response = await fetch(`${this.baseURL}/api/v1/students/${encodeURIComponent(id)}`, { credentials: "include", cache: "no-store" });
    if (response.status === 404) return null;
    if (!response.ok) throw new Error("Student API is unavailable");
    return response.json() as Promise<Student>;
  }
}
