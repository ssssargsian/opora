"use client";

import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, FilePenLine } from "lucide-react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useId, useState } from "react";

import { documentsAPI } from "@/lib/api/documents";

declare global {
  interface Window { DocsAPI?: { DocEditor: new (id: string, config: Record<string, unknown>) => { destroyEditor: () => void } } }
}

export default function DocumentEditorPage() {
  const { id } = useParams<{ id: string }>();
  const editorId = `onlyoffice-${useId().replaceAll(":", "")}`;
  const [scriptReady, setScriptReady] = useState(false);
  const editor = useQuery({ queryKey: ["document-editor", id], queryFn: () => documentsAPI.editor(id), enabled: Boolean(id) });

  useEffect(() => {
    if (!editor.data) return;
    const source = `${editor.data.documentServerUrl}/web-apps/apps/api/documents/api.js`;
    const existing = document.querySelector<HTMLScriptElement>(`script[src="${source}"]`);
    if (existing && window.DocsAPI) { queueMicrotask(() => setScriptReady(true)); return; }
    const script = existing ?? document.createElement("script"); script.src = source; script.async = true;
    script.addEventListener("load", () => setScriptReady(true), { once: true });
    script.addEventListener("error", () => setScriptReady(false), { once: true });
    if (!existing) document.head.appendChild(script);
  }, [editor.data]);

  useEffect(() => {
    if (!scriptReady || !editor.data || !window.DocsAPI) return;
    const instance = new window.DocsAPI.DocEditor(editorId, editor.data.config);
    return () => instance.destroyEditor();
  }, [editor.data, editorId, scriptReady]);

  return (
    <section className="editor-page">
      <header className="editor-toolbar"><Link className="back-link" href="/students"><ArrowLeft size={16} /> Вернуться к детям</Link><span><FilePenLine size={17} /> ONLYOFFICE · изменения сохраняются новой версией</span></header>
      {(editor.isPending || (editor.data && !scriptReady)) && <div className="editor-loading"><span className="loading-spinner" /> Подготавливаем защищённый редактор…</div>}
      {editor.isError && <div className="empty-state editor-error"><strong>Редактор недоступен</strong><span>Проверьте доступ и состояние ONLYOFFICE.</span><button type="button" onClick={() => void editor.refetch()}>Повторить</button></div>}
      <div id={editorId} className="onlyoffice-frame" aria-label="Редактор документа" />
    </section>
  );
}
