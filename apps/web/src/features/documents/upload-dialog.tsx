"use client";

import { useMutation } from "@tanstack/react-query";
import { FileUp, UploadCloud, X } from "lucide-react";
import { useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import { APIError } from "@/lib/api/client";
import { documentsAPI, formatBytes } from "@/lib/api/documents";

const MAX_BYTES = 25 * 1024 * 1024;

function uploadMessage(error: unknown): string {
  if (!(error instanceof APIError)) return "Не удалось загрузить документ";
  if (error.code === "unsupported_file") return "Неподдерживаемый формат файла";
  if (error.code === "file_too_large") return "Максимальный размер файла — 25 МБ";
  if (error.code === "unsafe_file") return "Файл не прошёл проверку безопасности";
  if (error.status === 403) return "Нет доступа для загрузки документа";
  if (error.status === 0) return "Нет связи с сервером. Попробуйте ещё раз";
  return "Сервис документов временно недоступен";
}

export function UploadDialog({ studentId, onClose, onUploaded }: { studentId: string; onClose: () => void; onUploaded: () => void }) {
  const input = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [title, setTitle] = useState("");
  const [documentType, setDocumentType] = useState("");
  const [confidentiality, setConfidentiality] = useState("standard");
  const [clientError, setClientError] = useState("");
  const upload = useMutation({
    mutationFn: async () => {
      if (!file) throw new Error("file_required");
      const form = new FormData(); form.set("file", file); if (title.trim()) form.set("title", title.trim());
      if (documentType.trim()) form.set("documentType", documentType.trim()); form.set("confidentialityLevel", confidentiality);
      return documentsAPI.upload(studentId, form);
    },
    onSuccess: onUploaded,
  });
  const choose = (next: File | null) => {
    setClientError("");
    if (!next) return;
    if (!/\.(docx|pdf)$/i.test(next.name)) { setClientError("Поддерживаются только DOCX и PDF"); return; }
    if (next.size > MAX_BYTES) { setClientError("Максимальный размер файла — 25 МБ"); return; }
    setFile(next); if (!title) setTitle(next.name.replace(/\.(docx|pdf)$/i, ""));
  };
  return (
    <div className="dialog-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !upload.isPending) onClose(); }}>
      <section className="dialog-card upload-dialog" role="dialog" aria-modal="true" aria-labelledby="upload-title">
        <header><span className="dialog-icon"><FileUp size={20} /></span><div><span className="eyebrow">Новый файл</span><h2 id="upload-title">Загрузить документ</h2></div><button type="button" className="icon-button" onClick={onClose} disabled={upload.isPending} aria-label="Закрыть"><X size={20} /></button></header>
        <form onSubmit={(event) => { event.preventDefault(); if (!file) { setClientError("Выберите файл"); return; } upload.mutate(); }}>
          <button className="drop-zone" type="button" onClick={() => input.current?.click()} onDragOver={(event) => event.preventDefault()} onDrop={(event) => { event.preventDefault(); choose(event.dataTransfer.files[0] ?? null); }}>
            <UploadCloud size={25} /><strong>{file ? file.name : "Перетащите DOCX или PDF сюда"}</strong><span>{file ? `${formatBytes(file.size)} · ${file.type || "тип определит сервер"}` : "или нажмите, чтобы выбрать · до 25 МБ"}</span>
          </button>
          <input ref={input} className="sr-only" type="file" accept=".docx,.pdf" onChange={(event) => choose(event.target.files?.[0] ?? null)} />
          <label><span>Название документа</span><input value={title} onChange={(event) => setTitle(event.target.value)} maxLength={255} placeholder="Например, Психологическое заключение" /></label>
          <label><span>Тип документа <small>необязательно</small></span><input value={documentType} onChange={(event) => setDocumentType(event.target.value)} maxLength={100} placeholder="Заключение, протокол, согласие…" /></label>
          <label><span>Конфиденциальность</span><select value={confidentiality} onChange={(event) => setConfidentiality(event.target.value)}><option value="standard">Стандартный доступ</option><option value="restricted">Ограниченный доступ</option></select></label>
          {(clientError || upload.isError) && <div className="form-error" role="alert">{clientError || uploadMessage(upload.error)}</div>}
          <footer><Button type="button" variant="ghost" onClick={onClose} disabled={upload.isPending}>Отмена</Button><Button type="submit" disabled={upload.isPending}>{upload.isPending ? "Проверяем и сохраняем…" : "Загрузить"}</Button></footer>
        </form>
      </section>
    </div>
  );
}
