"use client";

import { History, X } from "lucide-react";
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
      <section className="dialog-card" role="dialog" aria-modal="true" aria-labelledby="history-title">
        <header><span className="dialog-icon"><History size={20} /></span><div><span className="eyebrow">История версий</span><h2 id="history-title">{document.title}</h2></div><button type="button" className="icon-button" onClick={onClose} aria-label="Закрыть"><X size={20} /></button></header>
        <div className="version-list">
          {history.isPending && <div className="page-loading"><span className="loading-spinner" /> Загружаем версии…</div>}
          {history.isError && <div className="empty-state"><strong>История недоступна</strong><button type="button" onClick={() => void history.refetch()}>Повторить</button></div>}
          {history.data?.map((item, index) => <div className="version-row" key={item.id}><span className="version-dot" /><div><strong>Версия {item.versionNumber}</strong><span>{item.changedAt}</span></div><div><strong>{item.changedBy}</strong><span>{formatBytes(item.size)}</span></div><a className="version-download" href={documentsAPI.downloadURL(document.id, item.id)}>Скачать</a>{index === 0 && <span className="badge badge-green">Текущая</span>}</div>)}
        </div>
      </section>
    </div>
  );
}
