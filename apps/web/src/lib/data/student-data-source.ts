import type { Student, StudentSummary } from "./types";

export interface StudentDataSource {
  list(): Promise<StudentSummary[]>;
  get(id: string): Promise<Student | null>;
}
