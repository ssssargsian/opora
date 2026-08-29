"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useRouter } from "next/navigation";
import { ClipboardList, LogOut, School, Settings, UserRoundCog, UsersRound } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";

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
  const router = useRouter();
  const queryClient = useQueryClient();
  const user = useCurrentUser();
  const initials = user.displayName.split(" ").map((part) => part[0]).slice(0, 2).join("");

  return (
    <aside className="sidebar">
      <div className="brand-mark" aria-label="Опора">
        <span className="brand-icon"><School size={22} aria-hidden="true" /></span>
        <span><strong>Опора</strong><small>Школьная служба</small></span>
      </div>
      <nav className="sidebar-nav" aria-label="Основная навигация">
        {navigation.filter((item) => item.permissions.length === 0 || item.permissions.some((permission) => user.permissions.includes(permission))).map(({ href, label, icon: Icon }) => {
          const active = pathname === href || pathname.startsWith(`${href}/`);
          return (
            <Link key={href} href={href} aria-current={active ? "page" : undefined} className={cn("nav-item", active && "nav-item-active")}>
              <Icon size={19} aria-hidden="true" />
              {label}
            </Link>
          );
        })}
      </nav>
      <div className="sidebar-account">
        <span className="avatar">{initials}</span>
        <span><strong>{user.displayName}</strong><small>{user.organization.name}</small></span>
        <button type="button" className="sidebar-logout" aria-label="Выйти" title="Выйти" onClick={async () => { await authAPI.logout().catch(() => undefined); queryClient.clear(); router.replace("/login"); }}><LogOut size={16} /></button>
      </div>
    </aside>
  );
}
