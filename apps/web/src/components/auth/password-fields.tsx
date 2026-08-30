"use client";

import { Check, Circle, Eye, EyeOff, X } from "lucide-react";
import { useState, type InputHTMLAttributes, type ReactNode } from "react";
import { z } from "zod";

export const newPasswordSchema = z.string()
  .min(8, "Минимум 8 символов")
  .refine((value) => /\p{L}/u.test(value), "Добавьте хотя бы одну букву")
  .refine((value) => /\p{N}/u.test(value), "Добавьте хотя бы одну цифру");

export function PasswordInput({ label, error, ...input }: InputHTMLAttributes<HTMLInputElement> & { label: string; error?: string }) {
  const [visible, setVisible] = useState(false);
  return (
    <label>
      <span>{label}</span>
      <span className="password-input">
        <input {...input} type={visible ? "text" : "password"} aria-invalid={Boolean(error)} />
        <button type="button" onClick={() => setVisible((value) => !value)} aria-label={visible ? "Скрыть пароль" : "Показать пароль"}>
          {visible ? <EyeOff size={17} /> : <Eye size={17} />}
        </button>
      </span>
      {error && <small role="alert">{error}</small>}
    </label>
  );
}

function Requirement({ met, children }: { met: boolean; children: ReactNode }) {
  return <li className={met ? "is-met" : ""}>{met ? <Check size={14} /> : <Circle size={10} />}{children}</li>;
}

export function PasswordRequirements({ password, confirmation }: { password: string; confirmation?: string }) {
  const confirmationStarted = confirmation !== undefined && confirmation.length > 0;
  const passwordsMatch = confirmationStarted && password === confirmation;
  return (
    <div className="password-requirements" aria-live="polite">
      <strong>Пароль должен содержать:</strong>
      <ul>
        <Requirement met={Array.from(password).length >= 8}>минимум 8 символов</Requirement>
        <Requirement met={/\p{L}/u.test(password)}>хотя бы одну букву</Requirement>
        <Requirement met={/\p{N}/u.test(password)}>хотя бы одну цифру</Requirement>
        {confirmationStarted && <li className={passwordsMatch ? "is-met" : "is-error"}>{passwordsMatch ? <Check size={14} /> : <X size={14} />}{passwordsMatch ? "пароли совпадают" : "пароли не совпадают"}</li>}
      </ul>
    </div>
  );
}
