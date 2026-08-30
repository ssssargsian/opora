"use client";

import { CalendarDays, Download, FileClock, History, UserRound, X } from "lucide-react";
import { useEffect } from "react";

import { useQuery } from "@tanstack/react-query";

import { documentsAPI, formatBytes } from "@/lib/api/documents";
import type { StudentDocument } from "@/lib/data/types";

export function DocumentHistoryDialog({ document, onClose }: { document: StudentDocument; onClose: () => void }) {
  const history = useQuery({ queryKey: ["document-versions", document.id], queryFn: () => documentsAPI.versions(document.id) });
  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => { if (event.key === "Escape") onClose(); };
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [onClose]);

  return (
    <div className="dialog-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}>
      <section className="dialog-card history-dialog" role="dialog" aria-modal="true" aria-labelledby="history-title">
        <header><span className="dialog-icon"><History size={20} /></span><div><span className="eyebrow">История версий</span><h2 id="history-title">{document.title}</h2></div><button type="button" className="icon-button" onClick={onClose} aria-label="Закрыть"><X size={20} /></button></header>
        <div className="version-list">
          {history.isPending && <div className="page-loading"><span className="loading-spinner" /> Загружаем версии…</div>}
          {history.isError && <div className="empty-state"><strong>История недоступна</strong><button type="button" onClick={() => void history.refetch()}>Повторить</button></div>}
          {history.data?.map((item) => {
            const current = item.id === document.currentVersion.id;
            return <article className={`version-card${current ? " version-card-current" : ""}`} key={item.id}>
              <div className="version-heading"><span className="version-number"><FileClock size={17} />Версия {item.versionNumber}</span>{current && <span className="badge badge-green">Текущая версия</span>}</div>
              <div className="version-metadata"><span><CalendarDays size={14} /><span><small>Сохранена</small><strong>{item.changedAt}</strong></span></span><span><UserRound size={14} /><span><small>Автор изменений</small><strong>{item.changedBy}</strong></span></span><span><small>Размер</small><strong>{formatBytes(item.size)}</strong></span></div>
              <a className="version-download" href={documentsAPI.downloadURL(document.id, item.id)}><Download size={15} />Скачать</a>
            </article>;
          })}
          {history.data?.length === 0 && <div className="empty-state"><FileClock size={28} /><strong>Версий пока нет</strong></div>}
        </div>
      </section>
    </div>
  );
}
