import { MobileHeader, Sidebar } from "@/components/layout/sidebar";
import { AuthBoundary } from "@/features/auth/auth-boundary";

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  return (
    <AuthBoundary>
      <div className="app-shell">
        <Sidebar />
        <div className="dashboard-main"><MobileHeader /><main className="content-shell">{children}</main></div>
      </div>
    </AuthBoundary>
  );
}
