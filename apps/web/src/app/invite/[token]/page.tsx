"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { CheckCircle2 } from "lucide-react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useForm } from "react-hook-form";
import { z } from "zod";

import { Logo } from "@/components/brand/logo";
import { Button } from "@/components/ui/button";
import { APIError, authAPI } from "@/lib/api/client";

const schema = z.object({ password: z.string().min(12, "Минимум 12 символов"), confirmation: z.string() })
  .refine((value) => value.password === value.confirmation, { path: ["confirmation"], message: "Пароли не совпадают" });

export default function InvitationPage() {
  const { token } = useParams<{ token: string }>();
  const form = useForm<z.infer<typeof schema>>({ resolver: zodResolver(schema), defaultValues: { password: "", confirmation: "" } });
  const accept = useMutation({ mutationFn: (password: string) => authAPI.acceptInvitation(token, password) });
  const error = accept.error instanceof APIError && accept.error.code === "invitation_expired" ? "Срок действия приглашения истёк. Попросите администратора отправить новое."
    : accept.error instanceof APIError && accept.error.code === "invitation_used" ? "Это приглашение уже использовано."
      : "Приглашение недействительно или больше недоступно.";
  return <main className="login-page"><section className="login-card invitation-card">{accept.isSuccess ? <div className="invitation-success"><CheckCircle2 size={38} /><h1>Пароль установлен</h1><p>Учётная запись активирована. Теперь можно войти в Опору.</p><Button asChild><Link href="/login">Перейти ко входу</Link></Button></div> : <><div className="login-brand"><Logo /></div><span className="eyebrow">Приглашение специалиста</span><h1>Создайте пароль</h1><p>Ссылка одноразовая. После активации войдите с рабочим email.</p><form className="login-form" onSubmit={form.handleSubmit((values) => accept.mutate(values.password))}><label><span>Новый пароль</span><input type="password" autoComplete="new-password" autoFocus {...form.register("password")} />{form.formState.errors.password && <small>{form.formState.errors.password.message}</small>}</label><label><span>Повторите новый пароль</span><input type="password" autoComplete="new-password" {...form.register("confirmation")} />{form.formState.errors.confirmation && <small>{form.formState.errors.confirmation.message}</small>}</label>{accept.isError && <div className="form-error" role="alert">{error}</div>}<Button type="submit" disabled={accept.isPending}>{accept.isPending ? "Активируем…" : "Принять приглашение"}</Button></form></>}</section></main>;
}
