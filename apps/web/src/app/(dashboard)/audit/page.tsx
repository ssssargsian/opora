"use client";

import { useQuery } from "@tanstack/react-query";
import { ClipboardList, ShieldCheck } from "lucide-react";

import { Button } from "@/components/ui/button";
import { auditAPI } from "@/lib/api/audit";

const actionLabels: Record<string, string> = {
  "document.view": "Просмотр документа",
  "document.download": "Скачивание документа",
  "document.upload": "Загрузка документа",
  "document.edit": "Редактирование документа",
  "student.view": "Просмотр карточки",
  "student.create": "Создание карточки",
  "student.update": "Изменение карточки",
  "permission.grant": "Изменение доступа",
  "permission.revoke": "Отзыв доступа",
  "user.invite": "Создание пользователя",
  "user.role_change": "Изменение роли",
  "user.update": "Изменение специалиста",
};

const resourceLabels: Record<string, string> = {
  document: "Документ",
  student: "Ребёнок",
  user: "Пользователь",
  session: "Сессия",
};

export default function AuditPage() {
  const audit = useQuery({ queryKey: ["audit"], queryFn: auditAPI.list });
  return (
    <section>
      <header className="page-header"><div><span className="eyebrow">Контроль</span><h1>Журнал действий</h1><p>Неизменяемая история значимых действий в системе.</p></div><div className="audit-note"><ShieldCheck size={18} /><span>Записи доступны только для просмотра</span></div></header>
      {audit.isPending && <div className="data-panel page-loading"><span className="loading-spinner" />Загружаем журнал…</div>}
      {audit.isError && <div className="data-panel empty-state"><strong>Журнал временно недоступен</strong><Button variant="outline" onClick={() => void audit.refetch()}>Повторить</Button></div>}
      {audit.data && audit.data.length > 0 && <div className="data-panel table-scroll"><table><thead><tr><th>Дата</th><th>Пользователь</th><th>Действие</th><th>Объект</th></tr></thead><tbody>{audit.data.map((entry) => <tr key={entry.id}><td className="muted-cell">{new Intl.DateTimeFormat("ru-RU", { dateStyle: "medium", timeStyle: "short" }).format(new Date(entry.createdAt))}</td><td className="name-cell"><strong>{entry.actorName}</strong></td><td>{actionLabels[entry.action] ?? entry.action}</td><td><span className="badge">{resourceLabels[entry.resourceType] ?? entry.resourceType}</span></td></tr>)}</tbody></table></div>}
      {audit.data?.length === 0 && <div className="data-panel empty-state"><ClipboardList size={30} /><strong>В журнале пока нет событий</strong><span>Значимые действия появятся здесь автоматически.</span></div>}
    </section>
  );
}
