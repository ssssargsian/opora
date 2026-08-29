import type { Student, StudentSummary } from "../types";
import type { StudentDataSource } from "../student-data-source";

const students: Student[] = [
  {
    id: "0198f2aa-31a0-7f60-88d0-4ca7c65ef101",
    fullName: "Иванов Иван Иванович",
    className: "7А",
    birthDate: "14 марта 2013",
    documentCount: 4,
    updatedAt: "29.08.2026, 14:20",
    access: [
      { id: "access-1", name: "Анна Петрова", specialty: "Психолог", grants: ["Просмотр", "Редактирование", "Скачивание"] },
      { id: "access-2", name: "Мария Смирнова", specialty: "Логопед", grants: ["Просмотр"] },
      { id: "access-3", name: "Сергей Иванов", specialty: "Социальный педагог", grants: ["Просмотр", "Скачивание"] },
    ],
  },
  { id: "0198f2aa-31a0-7f60-88d0-4ca7c65ef102", fullName: "Соколова Алиса Максимовна", className: "5Б", birthDate: "2 июля 2015", documentCount: 3, updatedAt: "29.08.2026, 12:04", access: [] },
  { id: "0198f2aa-31a0-7f60-88d0-4ca7c65ef103", fullName: "Морозов Артём Сергеевич", className: "8В", birthDate: "19 ноября 2012", documentCount: 6, updatedAt: "28.08.2026, 16:48", access: [] },
  { id: "0198f2aa-31a0-7f60-88d0-4ca7c65ef104", fullName: "Кузнецова София Андреевна", className: "3А", birthDate: "8 января 2017", documentCount: 2, updatedAt: "28.08.2026, 11:20", access: [] },
  { id: "0198f2aa-31a0-7f60-88d0-4ca7c65ef105", fullName: "Орлов Михаил Романович", className: "6Г", birthDate: "21 мая 2014", documentCount: 5, updatedAt: "26.08.2026, 17:05", access: [] },
  { id: "0198f2aa-31a0-7f60-88d0-4ca7c65ef106", fullName: "Волкова Ева Дмитриевна", className: "9А", birthDate: "30 сентября 2011", documentCount: 4, updatedAt: "25.08.2026, 09:42", access: [] },
];

export class MockStudentDataSource implements StudentDataSource {
  async list(): Promise<StudentSummary[]> {
    return students.map((student) => ({
      id: student.id,
      fullName: student.fullName,
      className: student.className,
      documentCount: student.documentCount,
      updatedAt: student.updatedAt,
    }));
  }

  async get(id: string): Promise<Student | null> {
    return students.find((student) => student.id === id) ?? null;
  }
}
