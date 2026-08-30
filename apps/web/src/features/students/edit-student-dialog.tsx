"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { PencilLine, X } from "lucide-react";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";

import { Button } from "@/components/ui/button";
import { APIError } from "@/lib/api/client";
import { studentsAPI } from "@/lib/api/students";
import type { Student } from "@/lib/data/types";

const schema = z.object({
  lastName: z.string().trim().min(1, "Укажите фамилию").max(100),
  firstName: z.string().trim().min(1, "Укажите имя").max(100),
  middleName: z.string().trim().max(100),
  birthDate: z.string(),
  className: z.string().trim().max(32),
});
type Values = z.infer<typeof schema>;

export function EditStudentDialog({ student, onClose, onUpdated }: { student: Student; onClose: () => void; onUpdated: () => void }) {
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: {
    lastName: student.lastName ?? student.fullName.split(" ")[0] ?? "", firstName: student.firstName ?? student.fullName.split(" ")[1] ?? "", middleName: student.middleName ?? "",
    birthDate: student.birthDateValue ?? "", className: student.className === "—" ? "" : student.className,
  } });
  const update = useMutation({ mutationFn: (values: Values) => studentsAPI.update(student.id, values), onSuccess: onUpdated });
  useEffect(() => {
    const close = (event: KeyboardEvent) => { if (event.key === "Escape" && !update.isPending) onClose(); };
    document.addEventListener("keydown", close);
    return () => document.removeEventListener("keydown", close);
  }, [onClose, update.isPending]);
  const error = update.error instanceof APIError && update.error.status === 403
    ? "У вас нет права изменять карточку ребёнка" : "Не удалось сохранить изменения";

  return <div className="dialog-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !update.isPending) onClose(); }}>
    <section className="dialog-card form-dialog" role="dialog" aria-modal="true" aria-labelledby="edit-student-title">
      <header><span className="dialog-icon"><PencilLine size={20} /></span><div><span className="eyebrow">Карточка ребёнка</span><h2 id="edit-student-title">Редактировать данные</h2></div><button type="button" className="icon-button" onClick={onClose} disabled={update.isPending} aria-label="Закрыть"><X size={19} /></button></header>
      <form onSubmit={form.handleSubmit((values) => update.mutate(values))}>
        <div className="form-grid">
          <label><span>Фамилия <b>*</b></span><input autoFocus {...form.register("lastName")} aria-invalid={Boolean(form.formState.errors.lastName)} />{form.formState.errors.lastName && <small>{form.formState.errors.lastName.message}</small>}</label>
          <label><span>Имя <b>*</b></span><input {...form.register("firstName")} aria-invalid={Boolean(form.formState.errors.firstName)} />{form.formState.errors.firstName && <small>{form.formState.errors.firstName.message}</small>}</label>
          <label className="form-grid-wide"><span>Отчество</span><input {...form.register("middleName")} /></label>
          <label><span>Дата рождения</span><input type="date" {...form.register("birthDate")} /></label>
          <label><span>Класс</span><input {...form.register("className")} /></label>
        </div>
        {update.isError && <div className="form-error" role="alert">{error}</div>}
        <footer><Button type="button" variant="ghost" onClick={onClose} disabled={update.isPending}>Отмена</Button><Button type="submit" disabled={update.isPending}>{update.isPending ? "Сохраняем…" : "Сохранить"}</Button></footer>
      </form>
    </section>
  </div>;
}
