"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Building2, LockKeyhole, UserRound } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";

import { Button } from "@/components/ui/button";
import { useCurrentUser } from "@/features/auth/auth-boundary";
import { APIError, authAPI, organizationAPI, type CurrentUser } from "@/lib/api/client";

const profileSchema = z.object({ lastName: z.string().trim().min(1, "Укажите фамилию").max(100), firstName: z.string().trim().min(1, "Укажите имя").max(100), middleName: z.string().trim().max(100), email: z.email("Введите корректный email") });
const passwordSchema = z.object({ currentPassword: z.string().min(1, "Введите текущий пароль"), newPassword: z.string().min(12, "Минимум 12 символов"), confirmation: z.string() }).refine((value) => value.newPassword === value.confirmation, { path: ["confirmation"], message: "Пароли не совпадают" });
const organizationSchema = z.object({ name: z.string().trim().min(1, "Укажите название").max(255) });

export default function SettingsPage() {
  const user = useCurrentUser();
  const [toast, setToast] = useState("");
  const notify = (message: string) => { setToast(message); window.setTimeout(() => setToast(""), 3500); };
  return <section><header className="page-header"><div><span className="eyebrow">Учётная запись</span><h1>Настройки</h1><p>Личные данные, безопасность и параметры организации.</p></div></header><div className="settings-stack"><ProfileForm user={user} notify={notify} /><PasswordForm notify={notify} />{user.permissions.includes("organization.update") && <OrganizationForm name={user.organization.name} notify={notify} />}</div>{toast && <div className="toast" role="status">{toast}</div>}</section>;
}

function ProfileForm({ user, notify }: { user: CurrentUser; notify: (message: string) => void }) {
  const queryClient = useQueryClient();
  const form = useForm<z.infer<typeof profileSchema>>({ resolver: zodResolver(profileSchema), values: { lastName: user.lastName, firstName: user.firstName, middleName: user.middleName ?? "", email: user.email } });
  const mutation = useMutation({ mutationFn: authAPI.updateProfile, onSuccess: async () => { await queryClient.invalidateQueries({ queryKey: ["me"] }); notify("Профиль обновлён"); } });
  const error = mutation.error instanceof APIError && mutation.error.code === "email_exists" ? "Этот email уже используется" : "Не удалось обновить профиль";
  return <article className="settings-section"><header><span className="settings-icon"><UserRound size={20} /></span><div><h2>Мой профиль</h2><p>Ваши данные в интерфейсе Опоры.</p></div></header><form className="settings-form" onSubmit={form.handleSubmit((values) => mutation.mutate(values))}><div className="form-grid"><label><span>Фамилия</span><input autoComplete="family-name" {...form.register("lastName")} />{form.formState.errors.lastName && <small>{form.formState.errors.lastName.message}</small>}</label><label><span>Имя</span><input autoComplete="given-name" {...form.register("firstName")} />{form.formState.errors.firstName && <small>{form.formState.errors.firstName.message}</small>}</label><label><span>Отчество</span><input autoComplete="additional-name" {...form.register("middleName")} /></label><label><span>Email</span><input type="email" autoComplete="email" {...form.register("email")} />{form.formState.errors.email && <small>{form.formState.errors.email.message}</small>}</label></div>{mutation.isError && <div className="form-error" role="alert">{error}</div>}<footer><Button type="submit" disabled={mutation.isPending}>{mutation.isPending ? "Сохраняем…" : "Сохранить профиль"}</Button></footer></form></article>;
}

function PasswordForm({ notify }: { notify: (message: string) => void }) {
  const form = useForm<z.infer<typeof passwordSchema>>({ resolver: zodResolver(passwordSchema), defaultValues: { currentPassword: "", newPassword: "", confirmation: "" } });
  const mutation = useMutation({ mutationFn: ({ currentPassword, newPassword }: z.infer<typeof passwordSchema>) => authAPI.changePassword({ currentPassword, newPassword }), onSuccess: () => { form.reset(); notify("Пароль изменён, остальные сессии завершены"); } });
  const error = mutation.error instanceof APIError && mutation.error.code === "current_password_invalid" ? "Текущий пароль указан неверно" : "Не удалось изменить пароль";
  return <article className="settings-section"><header><span className="settings-icon"><LockKeyhole size={20} /></span><div><h2>Безопасность</h2><p>После смены пароля остальные активные сессии будут завершены.</p></div></header><form className="settings-form" onSubmit={form.handleSubmit((values) => mutation.mutate(values))}><div className="form-grid"><label className="form-grid-wide"><span>Текущий пароль</span><input type="password" autoComplete="current-password" {...form.register("currentPassword")} />{form.formState.errors.currentPassword && <small>{form.formState.errors.currentPassword.message}</small>}</label><label><span>Новый пароль</span><input type="password" autoComplete="new-password" {...form.register("newPassword")} />{form.formState.errors.newPassword && <small>{form.formState.errors.newPassword.message}</small>}</label><label><span>Повторите пароль</span><input type="password" autoComplete="new-password" {...form.register("confirmation")} />{form.formState.errors.confirmation && <small>{form.formState.errors.confirmation.message}</small>}</label></div>{mutation.isError && <div className="form-error" role="alert">{error}</div>}<footer><Button type="submit" disabled={mutation.isPending}>{mutation.isPending ? "Изменяем…" : "Сменить пароль"}</Button></footer></form></article>;
}

function OrganizationForm({ name, notify }: { name: string; notify: (message: string) => void }) {
  const queryClient = useQueryClient();
  const form = useForm<z.infer<typeof organizationSchema>>({ resolver: zodResolver(organizationSchema), values: { name } });
  const mutation = useMutation({ mutationFn: (values: { name: string }) => organizationAPI.update(values.name), onSuccess: async () => { await queryClient.invalidateQueries({ queryKey: ["me"] }); notify("Название организации обновлено"); } });
  return <article className="settings-section"><header><span className="settings-icon"><Building2 size={20} /></span><div><h2>Организация</h2><p>Название школы, отображаемое сотрудникам.</p></div></header><form className="settings-form" onSubmit={form.handleSubmit((values) => mutation.mutate(values))}><label><span>Название школы / организации</span><input {...form.register("name")} />{form.formState.errors.name && <small>{form.formState.errors.name.message}</small>}</label>{mutation.isError && <div className="form-error" role="alert">Не удалось обновить организацию</div>}<footer><Button type="submit" disabled={mutation.isPending}>{mutation.isPending ? "Сохраняем…" : "Сохранить организацию"}</Button></footer></form></article>;
}
