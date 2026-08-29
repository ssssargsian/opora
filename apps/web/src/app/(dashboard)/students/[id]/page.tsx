"use client";

import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, CalendarDays, GraduationCap } from "lucide-react";
import Link from "next/link";
import { useParams } from "next/navigation";

import { StudentWorkspace } from "@/features/students/student-workspace";
import { studentsAPI } from "@/lib/api/students";

export default function StudentPage() {
  const { id } = useParams<{ id: string }>();
  const student = useQuery({ queryKey: ["student", id], queryFn: () => studentsAPI.get(id), enabled: Boolean(id) });
  if (student.isPending) return <div className="data-panel page-loading"><span className="loading-spinner" /> Загружаем карточку…</div>;
  if (student.isError) return <div className="data-panel empty-state"><strong>Карточка недоступна</strong><Link href="/students">Вернуться к списку</Link></div>;

  return (
    <section>
      <Link className="back-link" href="/students"><ArrowLeft size={16} /> Все дети</Link>
      <header className="student-header"><div><span className="eyebrow">Карточка ребёнка</span><h1>{student.data.fullName}</h1><p>{student.data.className} класс</p></div><div className="student-facts"><div><CalendarDays size={18} /><span>Дата рождения<strong>{student.data.birthDate}</strong></span></div><div><GraduationCap size={18} /><span>Класс<strong>{student.data.className}</strong></span></div></div></header>
      <StudentWorkspace student={student.data} />
    </section>
  );
}
