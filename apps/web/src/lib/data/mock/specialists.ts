import type { Specialist } from "../types";

export const mockSpecialists: Specialist[] = [
  { id: "user-1", name: "Елена Воронцова", email: "e.vorontsova@school.local", role: "Администратор", status: "Активен" },
  { id: "user-2", name: "Анна Петрова", email: "a.petrova@school.local", role: "Психолог", status: "Активен" },
  { id: "user-3", name: "Мария Смирнова", email: "m.smirnova@school.local", role: "Логопед", status: "Активен" },
  { id: "user-4", name: "Сергей Иванов", email: "s.ivanov@school.local", role: "Социальный педагог", status: "Активен" },
  { id: "user-5", name: "Ольга Белова", email: "o.belova@school.local", role: "Специалист", status: "Приглашён" },
  { id: "user-6", name: "Дмитрий Крылов", email: "d.krylov@school.local", role: "Специалист", status: "Заблокирован" },
];
