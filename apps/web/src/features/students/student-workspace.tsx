"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Download, FileText, History, LockKeyhole, Settings2, Upload, UserRoundPlus } from "lucide-react";
import Link from "next/link";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { AccessDialog } from "@/features/access/access-dialog";
import { DocumentHistoryDialog } from "@/features/documents/document-history-dialog";
import { UploadDialog } from "@/features/documents/upload-dialog";
import { useCurrentUser } from "@/features/auth/auth-boundary";
import { accessAPI } from "@/lib/api/access";
import { documentsAPI, formatBytes } from "@/lib/api/documents";
import type { Student, StudentDocument } from "@/lib/data/types";

export function StudentWorkspace({ student }: { student: Student }) {
  const [tab, setTab] = useState<"documents" | "access">("documents");
  const [historyDocument, setHistoryDocument] = useState<StudentDocument | null>(null);
  const [uploadOpen, setUploadOpen] = useState(false);
  const [accessOpen, setAccessOpen] = useState(false);
  const [toast, setToast] = useState("");
  const queryClient = useQueryClient();
  const user = useCurrentUser();
  const documents = useQuery({ queryKey: ["student-documents", student.id], queryFn: () => documentsAPI.list(student.id) });
  const canUpload = user.permissions.includes("documents.upload");
  const canDownload = user.permissions.includes("documents.download");
  const canEdit = user.permissions.includes("documents.edit");
  const canViewAccess = user.permissions.includes("access.view");
  const canManageAccess = user.permissions.includes("access.manage");
  const access = useQuery({
    queryKey: ["student-access", student.id],
    queryFn: () => accessAPI.list(student.id),
    enabled: canViewAccess,
  });
  const notify = (message: string) => {
    setToast(message);
    window.setTimeout(() => setToast(""), 3500);
  };

  return (
    <>
      <div className="tabs" role="tablist" aria-label="Разделы карточки">
        <button type="button" role="tab" aria-selected={tab === "documents"} onClick={() => setTab("documents")}>Документы <span>{documents.data?.length ?? student.documentCount}</span></button>
        {canViewAccess && <button type="button" role="tab" aria-selected={tab === "access"} onClick={() => setTab("access")}>Доступ <span>{access.data?.length ?? 0}</span></button>}
      </div>
      {tab === "documents" ? (
        <section className="workspace-panel" role="tabpanel">
          <header className="panel-header"><div><h2>Документы {student.fullName}</h2><p>Все материалы хранятся здесь, каждое изменение сохраняется отдельной версией.</p></div>{canUpload && <Button type="button" onClick={() => setUploadOpen(true)}><Upload size={17} /> Загрузить документ</Button>}</header>
          <div className="document-list">
            {documents.isPending && <div className="page-loading"><span className="loading-spinner" /> Загружаем документы…</div>}
            {documents.isError && <div className="empty-state"><strong>Документы недоступны</strong><button type="button" onClick={() => void documents.refetch()}>Повторить</button></div>}
            {documents.data?.length === 0 && <div className="empty-state"><FileText size={28} /><strong>У ребёнка пока нет документов</strong><span>Загрузите первый DOCX или PDF в защищённое пространство.</span>{canUpload && <Button type="button" onClick={() => setUploadOpen(true)}><Upload size={17} />Загрузить документ</Button>}</div>}
            {documents.data?.map((document) => (
              <article className="document-row" key={document.id}>
                <span className={`file-icon file-${document.kind}`}><FileText size={21} /></span>
                <div className="document-title">
                  {document.kind === "docx" && canEdit
                    ? <Link className="document-open-link" href={`/documents/${document.id}/edit`} title={document.title}>{document.title}</Link>
                    : document.kind === "pdf"
                      ? <a className="document-open-link" href={documentsAPI.previewURL(document.id)} target="_blank" rel="noreferrer" title={document.title}>{document.title}</a>
                      : <strong title={document.title}>{document.title}</strong>}
                  <span><span className={`file-type-badge file-type-${document.kind}`}>{document.kind.toUpperCase()}</span><span>Версия {document.currentVersion.versionNumber}</span><span>{formatBytes(document.currentVersion.size)}</span>{document.confidentialityLevel === "restricted" && <span className="restricted-label"><LockKeyhole size={13} />Ограниченный доступ</span>}</span>
                </div>
                <div className="document-meta"><strong>{document.currentVersion.changedBy}</strong><span>{document.updatedAt}</span></div>
                <div className="document-actions">
                  {canDownload && <Button asChild variant="ghost" size="sm"><a href={documentsAPI.downloadURL(document.id)}><Download size={16} />Скачать</a></Button>}
                  <Button type="button" variant="ghost" size="sm" onClick={() => setHistoryDocument(document)}><History size={16} />История</Button>
                </div>
              </article>
            ))}
          </div>
        </section>
      ) : (
        <section className="workspace-panel" role="tabpanel">
          <header className="panel-header"><div><h2>Доступ к ребёнку</h2><p>Персональные разрешения специалистов.</p></div>{canManageAccess && <Button type="button" onClick={() => setAccessOpen(true)}><Settings2 size={17} />Настроить доступ</Button>}</header>
          <div className="access-list">
            {access.isPending && <div className="page-loading"><span className="loading-spinner" />Загружаем доступы…</div>}
            {access.isError && <div className="empty-state"><strong>Не удалось загрузить доступы</strong><Button variant="outline" onClick={() => void access.refetch()}>Повторить</Button></div>}
            {access.data?.length === 0 && <div className="empty-state"><UserRoundPlus size={29} /><strong>Доступ специалистам пока не настроен</strong><span>Назначьте сотрудника и выберите разрешения.</span>{canManageAccess && <Button type="button" onClick={() => setAccessOpen(true)}><Settings2 size={17} />Настроить доступ</Button>}</div>}
            {access.data?.map((entry) => <article className="access-row" key={entry.userId}><span className="person-avatar">{entry.displayName.split(" ").map((part) => part[0]).slice(0, 2).join("")}</span><div><strong title={entry.displayName}>{entry.displayName}</strong><span>{entry.roleName} · {entry.email}</span></div><div className="grant-list">{entry.grants.map((grant) => <span className="badge" key={grant}>{grantLabels[grant]}</span>)}</div></article>)}
          </div>
        </section>
      )}
      {uploadOpen && <UploadDialog studentId={student.id} onClose={() => setUploadOpen(false)} onUploaded={() => { setUploadOpen(false); notify("Документ проверен и сохранён"); void queryClient.invalidateQueries({ queryKey: ["student-documents", student.id] }); void queryClient.invalidateQueries({ queryKey: ["student", student.id] }); void queryClient.invalidateQueries({ queryKey: ["students"] }); }} />}
      {accessOpen && <AccessDialog studentId={student.id} assignments={access.data ?? []} onClose={() => setAccessOpen(false)} onSaved={(message) => { setAccessOpen(false); notify(message); void queryClient.invalidateQueries({ queryKey: ["student-access", student.id] }); }} />}
      {historyDocument && <DocumentHistoryDialog document={historyDocument} onClose={() => setHistoryDocument(null)} />}
      {toast && <div className="toast" role="status">{toast}</div>}
    </>
  );
}

const grantLabels = {
  view: "Просмотр",
  upload: "Загрузка",
  download: "Скачивание",
  edit: "Редактирование",
} as const;
