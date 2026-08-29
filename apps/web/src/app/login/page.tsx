import { LockKeyhole, ShieldCheck } from "lucide-react";

import { LoginForm } from "@/components/auth/login-form";

export default function LoginPage() {
  return (
    <main className="login-page">
      <section className="login-card" aria-labelledby="login-title">
        <div className="login-brand"><span className="login-lock"><LockKeyhole size={20} /></span><strong>Опора</strong></div>
        <h1 id="login-title">Вход в систему</h1>
        <p>Защищённое рабочее пространство школьных специалистов.</p>
        <LoginForm />
        {process.env.NODE_ENV === "development" && <div className="mock-credentials"><ShieldCheck size={17} /><span><strong>Локальная среда:</strong> admin@opora.local · пароль из DEV_ADMIN_PASSWORD</span></div>}
        <small className="login-help">Проблемы со входом? Обратитесь к администратору школы.</small>
      </section>
    </main>
  );
}
