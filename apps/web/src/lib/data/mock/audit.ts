import type { AuditEntry } from "../types";

export const mockAuditEntries: AuditEntry[] = [
  { id: "audit-1", occurredAt: "29.08.2026, 14:20", actor: "Анна Петрова", action: "Открыла документ", object: "Психологическое заключение" },
  { id: "audit-2", occurredAt: "29.08.2026, 12:04", actor: "Мария Смирнова", action: "Скачала документ", object: "Характеристика" },
  { id: "audit-3", occurredAt: "29.08.2026, 10:32", actor: "Елена Воронцова", action: "Изменила доступ", object: "Иванов Иван Иванович" },
  { id: "audit-4", occurredAt: "28.08.2026, 16:48", actor: "Сергей Иванов", action: "Просмотрел карточку", object: "Морозов Артём Сергеевич" },
  { id: "audit-5", occurredAt: "28.08.2026, 11:20", actor: "Анна Петрова", action: "Загрузила документ", object: "Протокол ППк" },
];
