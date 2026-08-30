import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { MobileHeader } from "./sidebar";

const { replace, clear, logout } = vi.hoisted(() => ({ replace: vi.fn(), clear: vi.fn(), logout: vi.fn().mockResolvedValue(undefined) }));

vi.mock("next/navigation", () => ({ usePathname: () => "/students", useRouter: () => ({ replace }) }));
vi.mock("@tanstack/react-query", () => ({ useQueryClient: () => ({ clear }) }));
vi.mock("@/features/auth/auth-boundary", () => ({ useCurrentUser: () => ({
  displayName: "Анна Петрова", organization: { name: "Школа" }, permissions: ["students.list"],
}) }));
vi.mock("@/lib/api/client", () => ({ authAPI: { logout } }));

describe("MobileHeader", () => {
  it("exposes backend logout from the mobile drawer", async () => {
    render(<MobileHeader />);
    fireEvent.click(screen.getByRole("button", { name: "Открыть меню" }));
    fireEvent.click(screen.getByRole("button", { name: "Выйти" }));
    await waitFor(() => expect(logout).toHaveBeenCalledOnce());
    expect(clear).toHaveBeenCalledOnce();
    expect(replace).toHaveBeenCalledWith("/login");
  });
});
