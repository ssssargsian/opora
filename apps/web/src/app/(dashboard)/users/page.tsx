"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { MailCheck, MailPlus, PencilLine, UsersRound, UserPlus } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { useCurrentUser } from "@/features/auth/auth-boundary";
import { CreateUserDialog } from "@/features/users/create-user-dialog";
import { EditUserDialog } from "@/features/users/edit-user-dialog";
import { APIError } from "@/lib/api/client";
import { usersAPI, type OrganizationUser } from "@/lib/api/users";

const statusLabel = { active: "Приглашение принято", invited: "Приглашение не принято", blocked: "Заблокирован" } as const;
const smtpErrors: Record<string, string> = {
  smtp_connection_failed: "Не удалось подключиться к почтовому серверу",
  smtp_tls_failed: "Не удалось установить защищённое SMTP-соединение",
  smtp_authentication_failed: "Почтовый сервер отклонил SMTP-авторизацию",
  smtp_sender_rejected: "Почтовый сервер отклонил адрес отправителя",
  smtp_recipient_rejected: "Почтовый сервер отклонил адрес получателя",
  smtp_send_failed: "Почтовый сервер не принял письмо",
};

function invitationDate(user: OrganizationUser) {
  const value = user.status === "active" ? user.invitationAcceptedAt : user.invitationCreatedAt ?? user.createdAt;
  if (!value) return null;
  return new Intl.DateTimeFormat("ru-RU", { day: "numeric", month: "short", year: "numeric" }).format(new Date(value));
}

export default function UsersPage() {
  const user = useCurrentUser();
  const queryClient = useQueryClient();
  const canView = user.permissions.includes("users.view") || user.permissions.includes("users.manage");
  const users = useQuery({ queryKey: ["users"], queryFn: usersAPI.list, enabled: canView });
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<OrganizationUser | null>(null);
  const [toast, setToast] = useState("");
  const canCreate = user.permissions.includes("users.create") || user.permissions.includes("users.invite") || user.permissions.includes("users.manage");
  const canManage = user.permissions.includes("users.manage");
  const created = (createdUser: OrganizationUser) => {
    setDialogOpen(false);
    setToast(createdUser.invitationDelivery === "failed" ? smtpErrors[createdUser.invitationError ?? ""] ?? "Учётная запись создана, но приглашение не отправлено" : "Пользователь создан, приглашение отправлено");
    void queryClient.invalidateQueries({ queryKey: ["users"] });
  };
  const resend = useMutation({ mutationFn: usersAPI.resendInvitation, onSuccess: () => { setToast("Новое приглашение отправлено"); void queryClient.invalidateQueries({ queryKey: ["users"] }); }, onError: (error) => setToast(error instanceof APIError ? smtpErrors[error.code] ?? "Не удалось отправить приглашение" : "Не удалось отправить приглашение") });
  return (
    <>
      <section>
        <header className="page-header"><div><span className="eyebrow">Команда</span><h1>Специалисты</h1><p>Сотрудники школы, их роли и состояние учётных записей.</p></div>{canCreate && <Button type="button" onClick={() => setDialogOpen(true)}><UserPlus size={18} />Добавить специалиста</Button>}</header>
        {users.isPending && canView && <div className="data-panel page-loading"><span className="loading-spinner" />Загружаем специалистов…</div>}
        {users.isError && <div className="data-panel empty-state"><strong>Не удалось загрузить специалистов</strong><Button type="button" variant="outline" onClick={() => void users.refetch()}>Повторить</Button></div>}
        {users.data && users.data.length > 0 && <div className="data-panel table-scroll"><table className="specialists-table"><thead><tr><th>ФИО</th><th>Email</th><th>Роль</th><th>Статус приглашения</th><th aria-label="Действия" /></tr></thead><tbody>{users.data.map((specialist) => <tr key={specialist.id}><td className="name-cell"><strong title={specialist.displayName}>{specialist.displayName}</strong></td><td className="muted-cell"><span className="cell-ellipsis" title={specialist.email}>{specialist.email}</span></td><td><span className="badge">{specialist.roleName}</span></td><td><div className="invitation-status"><span className={`status status-${specialist.status}`}>{specialist.status === "active" && <MailCheck size={13} />}{statusLabel[specialist.status]}</span>{specialist.status !== "blocked" && invitationDate(specialist) && <small>{specialist.status === "active" ? "Принято" : "Отправлено"} {invitationDate(specialist)}</small>}</div></td><td><div className="table-actions">{specialist.status === "invited" && canCreate && <Button size="sm" variant="ghost" disabled={resend.isPending} onClick={() => resend.mutate(specialist.id)}><MailPlus size={15} />Отправить снова</Button>}{canManage && <Button size="sm" variant="ghost" onClick={() => setEditing(specialist)}><PencilLine size={15} />Редактировать</Button>}</div></td></tr>)}</tbody></table></div>}
        {users.data?.length === 0 && <div className="data-panel empty-state"><UsersRound size={30} /><strong>Специалисты ещё не добавлены</strong><span>Добавьте сотрудника и назначьте ему роль.</span>{canCreate && <Button type="button" onClick={() => setDialogOpen(true)}><UserPlus size={17} />Добавить специалиста</Button>}</div>}
        {!canView && <div className="data-panel empty-state"><UsersRound size={30} /><strong>Список специалистов недоступен</strong><span>Вы можете создать учётную запись, но просмотр списка требует отдельного разрешения.</span></div>}
      </section>
      {dialogOpen && <CreateUserDialog onClose={() => setDialogOpen(false)} onCreated={created} />}
      {editing && <EditUserDialog user={editing} onClose={() => setEditing(null)} onUpdated={() => { setEditing(null); setToast("Данные специалиста обновлены"); void queryClient.invalidateQueries({ queryKey: ["users"] }); }} />}
      {toast && <div className="toast toast-wide" role="status">{toast}</div>}
    </>
  );
}
