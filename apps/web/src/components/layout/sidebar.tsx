"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { ClipboardList, LogOut, Menu, Settings, UserRoundCog, UsersRound, X } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { Logo } from "@/components/brand/logo";
import { useCurrentUser } from "@/features/auth/auth-boundary";
import { authAPI } from "@/lib/api/client";
import { cn } from "@/lib/cn";

const navigation = [
  { href: "/students", label: "Дети", icon: UsersRound, permissions: ["students.list"] },
  { href: "/users", label: "Специалисты", icon: UserRoundCog, permissions: ["users.view", "users.create", "users.invite", "users.manage"] },
  { href: "/audit", label: "Журнал действий", icon: ClipboardList, permissions: ["audit.view"] },
  { href: "/settings", label: "Настройки", icon: Settings, permissions: [] },
];

export function Sidebar() {
  const pathname = usePathname();
  const user = useCurrentUser();

  return (
    <aside className="sidebar">
      <div className="brand-mark"><Logo /></div>
      <Navigation pathname={pathname} permissions={user.permissions} />
      <Account compact />
    </aside>
  );
}

export function MobileHeader() {
  const pathname = usePathname();
  const user = useCurrentUser();
  const [open, setOpen] = useState(false);
  useEffect(() => {
    const close = (event: KeyboardEvent) => { if (event.key === "Escape") setOpen(false); };
    document.addEventListener("keydown", close);
    return () => document.removeEventListener("keydown", close);
  }, []);
  return <><header className="mobile-header"><Logo /><button type="button" className="mobile-menu-button" onClick={() => setOpen(true)} aria-label="Открыть меню" aria-expanded={open}><Menu size={22} /></button></header>{open && <div className="mobile-drawer-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setOpen(false); }}><aside className="mobile-drawer" role="dialog" aria-modal="true" aria-label="Навигация"><header><Logo /><button type="button" className="icon-button" autoFocus onClick={() => setOpen(false)} aria-label="Закрыть меню"><X size={21} /></button></header><Navigation pathname={pathname} permissions={user.permissions} onNavigate={() => setOpen(false)} /><Account /></aside></div>}</>;
}

function Navigation({ pathname, permissions, onNavigate }: { pathname: string; permissions: string[]; onNavigate?: () => void }) {
  return <nav className="sidebar-nav" aria-label="Основная навигация">{navigation.filter((item) => item.permissions.length === 0 || item.permissions.some((permission) => permissions.includes(permission))).map(({ href, label, icon: Icon }) => {
    const active = pathname === href || pathname.startsWith(`${href}/`);
    return <Link key={href} href={href} onClick={onNavigate} aria-current={active ? "page" : undefined} className={cn("nav-item", active && "nav-item-active")}><Icon size={19} aria-hidden="true" />{label}</Link>;
  })}</nav>;
}

function Account({ compact = false }: { compact?: boolean }) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const user = useCurrentUser();
  const [pending, setPending] = useState(false);
  const initials = user.displayName.split(" ").map((part) => part[0]).slice(0, 2).join("");
  const logout = async () => { setPending(true); try { await authAPI.logout(); queryClient.clear(); router.replace("/login"); } catch { setPending(false); } };
  return <div className={cn("sidebar-account", !compact && "mobile-account")}><span className="avatar">{initials}</span><span><strong>{user.displayName}</strong><small>{user.organization.name}</small></span><button type="button" className={cn("sidebar-logout", !compact && "mobile-logout")} aria-label="Выйти" title="Выйти" disabled={pending} onClick={() => void logout()}><LogOut size={16} />{!compact && <span>{pending ? "Выходим…" : "Выйти"}</span>}</button></div>;
}
