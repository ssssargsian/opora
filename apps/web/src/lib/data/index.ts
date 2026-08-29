import { HTTPStudentDataSource } from "./http/student-data-source";
import { MockStudentDataSource } from "./mock/students";
import type { StudentDataSource } from "./student-data-source";

export function getStudentDataSource(): StudentDataSource {
  const mockEnabled = process.env.NODE_ENV === "development" && process.env.NEXT_PUBLIC_DATA_MODE === "mock";
  if (mockEnabled) return new MockStudentDataSource();
  return new HTTPStudentDataSource(process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080");
}
