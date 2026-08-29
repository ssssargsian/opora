"use client";

import { useQuery } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { createContext, useContext, useEffect } from "react";

import { APIError, authAPI, type CurrentUser } from "@/lib/api/client";

const AuthContext = createContext<CurrentUser | null>(null);

export function useCurrentUser(): CurrentUser {
  const user = useContext(AuthContext);
  if (!user) throw new Error("useCurrentUser must be used inside AuthBoundary");
  return user;
}

export function AuthBoundary({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const query = useQuery({ queryKey: ["me"], queryFn: authAPI.me, retry: (count, error) => !(error instanceof APIError && error.status === 401) && count < 2 });

  useEffect(() => {
    if (query.error instanceof APIError && query.error.status === 401) router.replace("/login");
  }, [query.error, router]);

  if (query.isPending || (query.error instanceof APIError && query.error.status === 401)) {
    return <main className="auth-check"><span className="loading-spinner" /> Проверяем защищённую сессию…</main>;
  }
  if (query.isError) {
    return <main className="auth-check auth-error"><strong>Сервер временно недоступен</strong><span>Сессия не сброшена. Попробуйте ещё раз.</span><button type="button" onClick={() => void query.refetch()}>Повторить</button></main>;
  }
  return <AuthContext.Provider value={query.data}>{children}</AuthContext.Provider>;
}
