"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { UsersRound, UserPlus } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { useCurrentUser } from "@/features/auth/auth-boundary";
import { CreateUserDialog } from "@/features/users/create-user-dialog";
import { usersAPI, type OrganizationUser } from "@/lib/api/users";

const statusLabel = { active: "Активен", invited: "Ожидает активации", blocked: "Заблокирован" } as const;

export default function UsersPage() {
  const user = useCurrentUser();
  const queryClient = useQueryClient();
  const canView = user.permissions.includes("users.view") || user.permissions.includes("users.manage");
  const users = useQuery({ queryKey: ["users"], queryFn: usersAPI.list, enabled: canView });
  const [dialogOpen, setDialogOpen] = useState(false);
  const [toast, setToast] = useState("");
  const canCreate = user.permissions.includes("users.create") || user.permissions.includes("users.invite") || user.permissions.includes("users.manage");
  const created = (createdUser: OrganizationUser) => {
    setDialogOpen(false);
    setToast(createdUser.initialPassword
      ? `Пользователь создан. Начальный пароль: ${createdUser.initialPassword}`
      : "Пользователь создан");
    void queryClient.invalidateQueries({ queryKey: ["users"] });
  };
  return (
    <>
      <section>
        <header className="page-header"><div><span className="eyebrow">Команда</span><h1>Специалисты</h1><p>Сотрудники школы, их роли и состояние учётных записей.</p></div>{canCreate && <Button type="button" onClick={() => setDialogOpen(true)}><UserPlus size={18} />Добавить специалиста</Button>}</header>
        {users.isPending && canView && <div className="data-panel page-loading"><span className="loading-spinner" />Загружаем специалистов…</div>}
        {users.isError && <div className="data-panel empty-state"><strong>Не удалось загрузить специалистов</strong><Button type="button" variant="outline" onClick={() => void users.refetch()}>Повторить</Button></div>}
        {users.data && users.data.length > 0 && <div className="data-panel table-scroll"><table><thead><tr><th>ФИО</th><th>Email</th><th>Роль</th><th>Статус</th></tr></thead><tbody>{users.data.map((specialist) => <tr key={specialist.id}><td className="name-cell"><strong>{specialist.displayName}</strong></td><td className="muted-cell">{specialist.email}</td><td><span className="badge">{specialist.roleName}</span></td><td><span className={`status status-${specialist.status}`}>{statusLabel[specialist.status]}</span></td></tr>)}</tbody></table></div>}
        {users.data?.length === 0 && <div className="data-panel empty-state"><UsersRound size={30} /><strong>Специалисты ещё не добавлены</strong><span>Добавьте сотрудника и назначьте ему роль.</span>{canCreate && <Button type="button" onClick={() => setDialogOpen(true)}><UserPlus size={17} />Добавить специалиста</Button>}</div>}
        {!canView && <div className="data-panel empty-state"><UsersRound size={30} /><strong>Список специалистов недоступен</strong><span>Вы можете создать учётную запись, но просмотр списка требует отдельного разрешения.</span></div>}
      </section>
      {dialogOpen && <CreateUserDialog onClose={() => setDialogOpen(false)} onCreated={created} />}
      {toast && <div className="toast toast-wide" role="status">{toast}</div>}
    </>
  );
}
