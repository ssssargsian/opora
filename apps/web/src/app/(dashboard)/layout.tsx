import { Sidebar } from "@/components/layout/sidebar";
import { AuthBoundary } from "@/features/auth/auth-boundary";

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  return (
    <AuthBoundary>
      <div className="app-shell">
        <Sidebar />
        <main className="content-shell">{children}</main>
      </div>
    </AuthBoundary>
  );
}
