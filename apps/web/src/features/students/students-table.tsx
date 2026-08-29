"use client";

import { Search } from "lucide-react";
import Link from "next/link";
import { useDeferredValue, useState } from "react";

import type { StudentSummary } from "@/lib/data/types";

export function StudentsTable({ students }: { students: StudentSummary[] }) {
  const [query, setQuery] = useState("");
  const deferredQuery = useDeferredValue(query.trim().toLocaleLowerCase("ru"));
  const filtered = students.filter((student) => `${student.fullName} ${student.className}`.toLocaleLowerCase("ru").includes(deferredQuery));

  return (
    <>
      <div className="toolbar">
        <label className="search-field"><Search size={18} /><span className="sr-only">Поиск</span><input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Поиск по ФИО или классу" /></label>
        <span className="results-count">Найдено: {filtered.length}</span>
      </div>
      <div className="data-panel table-scroll">
        <table>
          <thead><tr><th>ФИО</th><th>Класс</th><th>Документы</th><th>Последнее изменение</th></tr></thead>
          <tbody>
            {filtered.map((student) => (
              <tr key={student.id}>
                <td><Link className="student-link" href={`/students/${student.id}`}>{student.fullName}</Link></td>
                <td><span className="class-badge">{student.className}</span></td>
                <td>{student.documentCount}</td>
                <td className="muted-cell">{student.updatedAt}</td>
              </tr>
            ))}
            {filtered.length === 0 && <tr><td colSpan={4}><div className="empty-state"><strong>Ничего не найдено</strong><span>Проверьте ФИО или номер класса.</span></div></td></tr>}
          </tbody>
        </table>
      </div>
    </>
  );
}
