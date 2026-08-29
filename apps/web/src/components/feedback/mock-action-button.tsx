"use client";

import { useState } from "react";

import { Button } from "@/components/ui/button";

export function MockActionButton({ children, variant = "default", size = "default" }: {
  children: React.ReactNode;
  variant?: "default" | "outline" | "ghost";
  size?: "default" | "sm" | "icon";
}) {
  const [notice, setNotice] = useState(false);
  return (
    <span className="mock-action">
      <Button type="button" variant={variant} size={size} onClick={() => setNotice(true)}>{children}</Button>
      {notice && <span className="mock-action-notice" role="status">Действие доступно после подключения API.<button type="button" onClick={() => setNotice(false)} aria-label="Закрыть">×</button></span>}
    </span>
  );
}
