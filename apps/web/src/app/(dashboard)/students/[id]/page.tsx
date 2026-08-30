"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, CalendarDays, GraduationCap, PencilLine } from "lucide-react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { useCurrentUser } from "@/features/auth/auth-boundary";
import { EditStudentDialog } from "@/features/students/edit-student-dialog";
import { StudentWorkspace } from "@/features/students/student-workspace";
import { studentsAPI } from "@/lib/api/students";

export default function StudentPage() {
  const { id } = useParams<{ id: string }>();
  const user = useCurrentUser();
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [toast, setToast] = useState("");
  const student = useQuery({ queryKey: ["student", id], queryFn: () => studentsAPI.get(id), enabled: Boolean(id) });
  if (student.isPending) return <div className="data-panel page-loading"><span className="loading-spinner" /> Загружаем карточку…</div>;
  if (student.isError) return <div className="data-panel empty-state"><strong>Карточка недоступна</strong><Link href="/students">Вернуться к списку</Link></div>;

  return (
    <section>
      <Link className="back-link" href="/students"><ArrowLeft size={16} /> Все дети</Link>
      <header className="student-header"><div><span className="eyebrow">Карточка ребёнка</span><h1>{student.data.fullName}</h1><p>{student.data.className} класс</p>{user.permissions.includes("students.update") && <Button className="student-edit" variant="outline" size="sm" onClick={() => setEditing(true)}><PencilLine size={16} />Редактировать</Button>}</div><div className="student-facts"><div><CalendarDays size={18} /><span>Дата рождения<strong>{student.data.birthDate}</strong></span></div><div><GraduationCap size={18} /><span>Класс<strong>{student.data.className}</strong></span></div></div></header>
      <StudentWorkspace student={student.data} />
      {editing && <EditStudentDialog student={student.data} onClose={() => setEditing(false)} onUpdated={() => { setEditing(false); setToast("Карточка ребёнка обновлена"); void queryClient.invalidateQueries({ queryKey: ["student", id] }); void queryClient.invalidateQueries({ queryKey: ["students"] }); window.setTimeout(() => setToast(""), 3000); }} />}
      {toast && <div className="toast" role="status">{toast}</div>}
    </section>
  );
}
