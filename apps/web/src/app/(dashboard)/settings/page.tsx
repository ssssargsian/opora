"use client";

import { Building2, ShieldCheck } from "lucide-react";

import { useCurrentUser } from "@/features/auth/auth-boundary";

export default function SettingsPage() {
  const user = useCurrentUser();
  return (
    <section>
      <header className="page-header"><div><span className="eyebrow">Организация</span><h1>Настройки</h1><p>Основная информация образовательной организации.</p></div></header>
      <div className="settings-grid">
        <article className="settings-card"><span className="settings-icon"><Building2 size={21} /></span><div><span className="field-label">Название</span><h2>{user.organization.name}</h2><div className="settings-line">ID организации: {user.organization.id}</div></div></article>
        <article className="settings-card security-card"><span className="settings-icon"><ShieldCheck size={21} /></span><div><span className="field-label">Контур данных</span><h2>Локальная инфраструктура</h2><p>Документы хранятся в закрытом S3 bucket. Доступ проверяется backend.</p></div></article>
      </div>
    </section>
  );
}
