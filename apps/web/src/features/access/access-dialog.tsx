"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { Check, Settings2, ShieldCheck, UserRound, X } from "lucide-react";
import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { accessAPI, type StudentAssignment, type StudentGrant } from "@/lib/api/access";
import { APIError } from "@/lib/api/client";
import { usersAPI } from "@/lib/api/users";

const grantOptions: { code: StudentGrant; label: string; description: string }[] = [
  { code: "view", label: "Просмотр", description: "Карточка ребёнка и список документов" },
  { code: "upload", label: "Загрузка документов", description: "Добавление DOCX и PDF" },
  { code: "download", label: "Скачивание", description: "Получение файлов и старых версий" },
  { code: "edit", label: "Редактирование", description: "Работа с DOCX в ONLYOFFICE" },
];

export function AccessDialog({
  studentId,
  assignments,
  onClose,
  onSaved,
}: {
  studentId: string;
  assignments: StudentAssignment[];
  onClose: () => void;
  onSaved: (message: string) => void;
}) {
  const users = useQuery({ queryKey: ["users"], queryFn: usersAPI.list });
  const [userId, setUserId] = useState(assignments[0]?.userId ?? "");
  const [grants, setGrants] = useState<StudentGrant[]>(assignments[0]?.grants ?? ["view"]);
  const activeUsers = users.data?.filter((item) => item.status === "active") ?? [];
  const selectedUserId = userId || activeUsers[0]?.id || "";
  const selectedUser = activeUsers.find((item) => item.id === selectedUserId);
  const selectedAssignment = assignments.find((item) => item.userId === selectedUserId);
  const save = useMutation({
    mutationFn: () => accessAPI.set(studentId, selectedUserId, grants),
    onSuccess: () => onSaved(selectedAssignment ? "Разрешения обновлены" : "Специалисту назначен доступ"),
  });
  const revoke = useMutation({
    mutationFn: () => accessAPI.revoke(studentId, selectedUserId),
    onSuccess: () => onSaved("Доступ отозван"),
  });
  const pending = save.isPending || revoke.isPending;
  useEffect(() => {
    const close = (event: KeyboardEvent) => { if (event.key === "Escape" && !pending) onClose(); };
    document.addEventListener("keydown", close);
    return () => document.removeEventListener("keydown", close);
  }, [onClose, pending]);

  const chooseUser = (next: string) => {
    setUserId(next);
    setGrants(assignments.find((item) => item.userId === next)?.grants ?? ["view"]);
    save.reset();
    revoke.reset();
  };
  const toggle = (grant: StudentGrant) => {
    setGrants((current) => {
      if (grant === "view" && current.some((item) => item !== "view")) return current;
      if (current.includes(grant)) return current.filter((item) => item !== grant);
      return grant === "view" ? [...current, grant] : Array.from(new Set([...current, "view", grant]));
    });
  };
  const error = save.error ?? revoke.error;
  const errorMessage = error instanceof APIError && error.status === 403
    ? "У вас нет права изменять доступ"
    : "Не удалось сохранить настройки доступа";

  return (
    <div className="dialog-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !pending) onClose(); }}>
      <section className="dialog-card access-dialog" role="dialog" aria-modal="true" aria-labelledby="access-dialog-title">
        <header><span className="dialog-icon"><Settings2 size={20} /></span><div><span className="eyebrow">Персональные права</span><h2 id="access-dialog-title">Настроить доступ</h2></div><button type="button" className="icon-button" onClick={onClose} disabled={pending} aria-label="Закрыть"><X size={19} /></button></header>
        <div className="access-form">
          <div className="access-editor">
            <section className="specialist-picker" aria-label="Специалисты">
              <div className="access-section-heading"><UserRound size={16} /><span>Специалисты</span></div>
              {users.isPending && <div className="access-picker-state"><span className="loading-spinner" />Загружаем…</div>}
              {activeUsers.map((item) => {
                const selected = item.id === selectedUserId;
                const assigned = assignments.some((entry) => entry.userId === item.id);
                return <button autoFocus={selected} type="button" key={item.id} className="specialist-option" aria-pressed={selected} onClick={() => chooseUser(item.id)} disabled={pending}>
                  <span className="person-avatar">{item.displayName.split(" ").map((part) => part[0]).slice(0, 2).join("")}</span>
                  <span><strong>{item.displayName}</strong><small>{item.roleName}</small></span>
                  {assigned && <span className="assigned-mark" title="Доступ настроен"><Check size={14} /></span>}
                </button>;
              })}
              {!users.isPending && activeUsers.length === 0 && <div className="access-picker-state">Нет активных специалистов</div>}
            </section>
            <section className="permission-panel">
              <div className="access-section-heading"><ShieldCheck size={16} /><span>{selectedUser ? `Права: ${selectedUser.displayName}` : "Разрешения"}</span></div>
              <fieldset disabled={!selectedUserId || pending}>{grantOptions.map((option) => <label className="permission-option" key={option.code}><input type="checkbox" checked={grants.includes(option.code)} disabled={pending || (option.code === "view" && grants.some((item) => item !== "view"))} onChange={() => toggle(option.code)} /><span><strong>{option.label}</strong><small>{option.description}</small></span></label>)}</fieldset>
            </section>
          </div>
          {users.isError && <div className="form-error" role="alert">Не удалось загрузить специалистов</div>}
          {(save.isError || revoke.isError) && <div className="form-error" role="alert">{errorMessage}</div>}
          <footer>{selectedAssignment && <Button type="button" variant="danger" onClick={() => revoke.mutate()} disabled={pending}>Отозвать доступ</Button>}<span className="footer-spacer" /><Button type="button" variant="ghost" onClick={onClose} disabled={pending}>Отмена</Button><Button type="button" onClick={() => save.mutate()} disabled={!selectedUserId || grants.length === 0 || pending}>{save.isPending ? "Сохраняем…" : "Сохранить"}</Button></footer>
        </div>
      </section>
    </div>
  );
}
