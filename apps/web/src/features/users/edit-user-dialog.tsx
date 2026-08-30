"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQuery } from "@tanstack/react-query";
import { PencilLine, X } from "lucide-react";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";

import { Button } from "@/components/ui/button";
import { APIError } from "@/lib/api/client";
import { usersAPI, type OrganizationUser } from "@/lib/api/users";

const schema = z.object({
  lastName: z.string().trim().min(1, "Укажите фамилию").max(100),
  firstName: z.string().trim().min(1, "Укажите имя").max(100),
  middleName: z.string().trim().max(100),
  email: z.email("Укажите корректный email").max(320),
  roleKey: z.string().min(1, "Выберите роль"),
});
type FormValues = z.infer<typeof schema>;

export function EditUserDialog({ user, onClose, onUpdated }: { user: OrganizationUser; onClose: () => void; onUpdated: (user: OrganizationUser) => void }) {
  const roles = useQuery({ queryKey: ["roles"], queryFn: usersAPI.roles });
  const form = useForm<FormValues>({ resolver: zodResolver(schema), defaultValues: {
    lastName: user.lastName, firstName: user.firstName, middleName: user.middleName ?? "", email: user.email, roleKey: user.roleKey,
  } });
  const update = useMutation({ mutationFn: (values: FormValues) => usersAPI.update(user.id, values), onSuccess: onUpdated });
  useEffect(() => {
    const close = (event: KeyboardEvent) => { if (event.key === "Escape" && !update.isPending) onClose(); };
    document.addEventListener("keydown", close);
    return () => document.removeEventListener("keydown", close);
  }, [onClose, update.isPending]);
  const error = update.error instanceof APIError && update.error.code === "email_exists"
    ? "Этот email уже используется"
    : update.error instanceof APIError && update.error.status === 403
      ? "У вас нет права изменять пользователей"
      : "Не удалось сохранить специалиста";

  return <div className="dialog-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !update.isPending) onClose(); }}>
    <section className="dialog-card form-dialog" role="dialog" aria-modal="true" aria-labelledby="edit-user-title">
      <header><span className="dialog-icon"><PencilLine size={20} /></span><div><span className="eyebrow">Учётная запись</span><h2 id="edit-user-title">Редактировать специалиста</h2></div><button type="button" className="icon-button" onClick={onClose} disabled={update.isPending} aria-label="Закрыть"><X size={19} /></button></header>
      <form onSubmit={form.handleSubmit((values) => update.mutate(values))}>
        <div className="form-grid">
          <label><span>Фамилия <b>*</b></span><input autoFocus autoComplete="family-name" {...form.register("lastName")} aria-invalid={Boolean(form.formState.errors.lastName)} />{form.formState.errors.lastName && <small>{form.formState.errors.lastName.message}</small>}</label>
          <label><span>Имя <b>*</b></span><input autoComplete="given-name" {...form.register("firstName")} aria-invalid={Boolean(form.formState.errors.firstName)} />{form.formState.errors.firstName && <small>{form.formState.errors.firstName.message}</small>}</label>
          <label className="form-grid-wide"><span>Отчество</span><input autoComplete="additional-name" {...form.register("middleName")} /></label>
          <label className="form-grid-wide"><span>Email <b>*</b></span><input type="email" autoComplete="email" {...form.register("email")} aria-invalid={Boolean(form.formState.errors.email)} />{form.formState.errors.email && <small>{form.formState.errors.email.message}</small>}</label>
          <label className="form-grid-wide"><span>Роль <b>*</b></span><select {...form.register("roleKey")} disabled={roles.isPending} aria-invalid={Boolean(form.formState.errors.roleKey)}>{roles.data?.map((role) => <option key={role.id} value={role.key}>{role.name}</option>)}</select>{form.formState.errors.roleKey && <small>{form.formState.errors.roleKey.message}</small>}</label>
        </div>
        {roles.isError && <div className="form-error" role="alert">Не удалось загрузить роли</div>}
        {update.isError && <div className="form-error" role="alert">{error}</div>}
        <footer><Button type="button" variant="ghost" onClick={onClose} disabled={update.isPending}>Отмена</Button><Button type="submit" disabled={update.isPending || roles.isPending}>{update.isPending ? "Сохраняем…" : "Сохранить"}</Button></footer>
      </form>
    </section>
  </div>;
}
