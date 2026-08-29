"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { UsersRound, UserPlus } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { useCurrentUser } from "@/features/auth/auth-boundary";
import { CreateStudentDialog } from "@/features/students/create-student-dialog";
import { StudentsTable } from "@/features/students/students-table";
import { studentsAPI } from "@/lib/api/students";

export default function StudentsPage() {
  const students = useQuery({ queryKey: ["students"], queryFn: studentsAPI.list });
  const queryClient = useQueryClient();
  const user = useCurrentUser();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [toast, setToast] = useState("");
  const canCreate = user.permissions.includes("students.create");
  const created = () => {
    setDialogOpen(false);
    setToast("Карточка ребёнка создана");
    void queryClient.invalidateQueries({ queryKey: ["students"] });
    window.setTimeout(() => setToast(""), 3500);
  };
  return (
    <>
    <section>
      <header className="page-header">
        <div><span className="eyebrow">Сопровождение</span><h1>Дети</h1><p>Карточки учащихся и связанные документы.</p></div>
        {canCreate && <Button type="button" onClick={() => setDialogOpen(true)}><UserPlus size={18} />Добавить ребёнка</Button>}
      </header>
      {students.isPending && <div className="data-panel page-loading"><span className="loading-spinner" /> Загружаем карточки…</div>}
      {students.isError && <div className="data-panel empty-state"><strong>Не удалось загрузить список</strong><button type="button" onClick={() => void students.refetch()}>Повторить</button></div>}
      {students.data && students.data.length > 0 && <StudentsTable students={students.data} />}
      {students.data?.length === 0 && <div className="data-panel empty-state"><UsersRound size={30} /><strong>В системе пока нет детей</strong><span>Создайте первую карточку для начала работы.</span>{canCreate && <Button type="button" onClick={() => setDialogOpen(true)}><UserPlus size={17} />Добавить ребёнка</Button>}</div>}
    </section>
    {dialogOpen && <CreateStudentDialog onClose={() => setDialogOpen(false)} onCreated={created} />}
    {toast && <div className="toast" role="status">{toast}</div>}
    </>
  );
}
