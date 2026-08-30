"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQuery } from "@tanstack/react-query";
import { UserPlus, X } from "lucide-react";
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

export function CreateUserDialog({ onClose, onCreated }: { onClose: () => void; onCreated: (user: OrganizationUser) => void }) {
  const roles = useQuery({ queryKey: ["roles"], queryFn: usersAPI.roles });
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { lastName: "", firstName: "", middleName: "", email: "", roleKey: "" },
  });
  const create = useMutation({ mutationFn: usersAPI.create, onSuccess: onCreated });
  useEffect(() => {
    const close = (event: KeyboardEvent) => { if (event.key === "Escape" && !create.isPending) onClose(); };
    document.addEventListener("keydown", close);
    return () => document.removeEventListener("keydown", close);
  }, [create.isPending, onClose]);

  const errorMessage = create.error instanceof APIError && create.error.code === "user_exists"
    ? "Пользователь с таким email уже состоит в организации"
    : create.error instanceof APIError && create.error.status === 403
      ? "У вас нет права добавлять пользователей"
      : "Не удалось создать пользователя";

  return (
    <div className="dialog-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !create.isPending) onClose(); }}>
      <section className="dialog-card form-dialog" role="dialog" aria-modal="true" aria-labelledby="create-user-title">
        <header><span className="dialog-icon"><UserPlus size={20} /></span><div><span className="eyebrow">Новая учётная запись</span><h2 id="create-user-title">Добавить специалиста</h2></div><button type="button" className="icon-button" onClick={onClose} disabled={create.isPending} aria-label="Закрыть"><X size={19} /></button></header>
        <form onSubmit={form.handleSubmit((values) => create.mutate(values))}>
          <div className="form-grid">
            <label><span>Фамилия <b>*</b></span><input autoFocus autoComplete="family-name" {...form.register("lastName")} aria-invalid={Boolean(form.formState.errors.lastName)} />{form.formState.errors.lastName && <small role="alert">{form.formState.errors.lastName.message}</small>}</label>
            <label><span>Имя <b>*</b></span><input autoComplete="given-name" {...form.register("firstName")} aria-invalid={Boolean(form.formState.errors.firstName)} />{form.formState.errors.firstName && <small role="alert">{form.formState.errors.firstName.message}</small>}</label>
            <label className="form-grid-wide"><span>Отчество</span><input autoComplete="additional-name" {...form.register("middleName")} /></label>
            <label className="form-grid-wide"><span>Email <b>*</b></span><input type="email" autoComplete="email" placeholder="specialist@school.local" {...form.register("email")} aria-invalid={Boolean(form.formState.errors.email)} />{form.formState.errors.email && <small role="alert">{form.formState.errors.email.message}</small>}</label>
            <label className="form-grid-wide"><span>Роль <b>*</b></span><select {...form.register("roleKey")} aria-invalid={Boolean(form.formState.errors.roleKey)} disabled={roles.isPending}><option value="">Выберите роль</option>{roles.data?.map((role) => <option key={role.id} value={role.key}>{role.name}</option>)}</select>{form.formState.errors.roleKey && <small role="alert">{form.formState.errors.roleKey.message}</small>}</label>
          </div>
          {roles.isError && <div className="form-error" role="alert">Не удалось загрузить роли</div>}
          {create.isError && <div className="form-error" role="alert">{errorMessage}</div>}
          <p className="form-hint">Специалист получит одноразовую ссылку и самостоятельно задаст пароль. Ссылка действует 48 часов.</p>
          <footer><Button type="button" variant="ghost" onClick={onClose} disabled={create.isPending}>Отмена</Button><Button type="submit" disabled={create.isPending || roles.isPending}>{create.isPending ? "Создаём…" : "Создать пользователя"}</Button></footer>
        </form>
      </section>
    </div>
  );
}
