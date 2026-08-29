"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { UserPlus, X } from "lucide-react";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";

import { Button } from "@/components/ui/button";
import { APIError } from "@/lib/api/client";
import { studentsAPI } from "@/lib/api/students";

const schema = z.object({
  lastName: z.string().trim().min(1, "Укажите фамилию").max(100, "Не более 100 символов"),
  firstName: z.string().trim().min(1, "Укажите имя").max(100, "Не более 100 символов"),
  middleName: z.string().trim().max(100, "Не более 100 символов"),
  birthDate: z.string(),
  className: z.string().trim().max(32, "Не более 32 символов"),
});

type FormValues = z.infer<typeof schema>;

export function CreateStudentDialog({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { lastName: "", firstName: "", middleName: "", birthDate: "", className: "" },
  });
  const create = useMutation({
    mutationFn: studentsAPI.create,
    onSuccess: onCreated,
  });
  useEffect(() => {
    const close = (event: KeyboardEvent) => { if (event.key === "Escape" && !create.isPending) onClose(); };
    document.addEventListener("keydown", close);
    return () => document.removeEventListener("keydown", close);
  }, [create.isPending, onClose]);

  const errorMessage = create.error instanceof APIError && create.error.status === 403
    ? "У вас нет права создавать карточки детей"
    : "Не удалось создать карточку. Проверьте данные и повторите попытку";

  return (
    <div className="dialog-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !create.isPending) onClose(); }}>
      <section className="dialog-card form-dialog" role="dialog" aria-modal="true" aria-labelledby="create-student-title">
        <header><span className="dialog-icon"><UserPlus size={20} /></span><div><span className="eyebrow">Новая карточка</span><h2 id="create-student-title">Добавить ребёнка</h2></div><button type="button" className="icon-button" onClick={onClose} disabled={create.isPending} aria-label="Закрыть"><X size={19} /></button></header>
        <form onSubmit={form.handleSubmit((values) => create.mutate(values))}>
          <div className="form-grid">
            <label><span>Фамилия <b>*</b></span><input autoFocus autoComplete="family-name" {...form.register("lastName")} aria-invalid={Boolean(form.formState.errors.lastName)} />{form.formState.errors.lastName && <small role="alert">{form.formState.errors.lastName.message}</small>}</label>
            <label><span>Имя <b>*</b></span><input autoComplete="given-name" {...form.register("firstName")} aria-invalid={Boolean(form.formState.errors.firstName)} />{form.formState.errors.firstName && <small role="alert">{form.formState.errors.firstName.message}</small>}</label>
            <label className="form-grid-wide"><span>Отчество</span><input autoComplete="additional-name" {...form.register("middleName")} aria-invalid={Boolean(form.formState.errors.middleName)} />{form.formState.errors.middleName && <small role="alert">{form.formState.errors.middleName.message}</small>}</label>
            <label><span>Дата рождения</span><input type="date" {...form.register("birthDate")} /></label>
            <label><span>Класс</span><input placeholder="Например, 7А" {...form.register("className")} aria-invalid={Boolean(form.formState.errors.className)} />{form.formState.errors.className && <small role="alert">{form.formState.errors.className.message}</small>}</label>
          </div>
          {create.isError && <div className="form-error" role="alert">{errorMessage}</div>}
          <footer><Button type="button" variant="ghost" onClick={onClose} disabled={create.isPending}>Отмена</Button><Button type="submit" disabled={create.isPending}>{create.isPending ? "Создаём…" : "Создать карточку"}</Button></footer>
        </form>
      </section>
    </div>
  );
}
