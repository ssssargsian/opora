import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { AccessDialog } from "./access-dialog";

const { listUsers, setAccess, revokeAccess } = vi.hoisted(() => ({
  listUsers: vi.fn(), setAccess: vi.fn(), revokeAccess: vi.fn(),
}));

vi.mock("@/lib/api/users", () => ({ usersAPI: { list: listUsers } }));
vi.mock("@/lib/api/access", () => ({
  accessAPI: { set: setAccess, revoke: revokeAccess },
}));

describe("AccessDialog", () => {
  it("assigns additive student grants to a selected specialist", async () => {
    listUsers.mockResolvedValue([{ id: "user-1", displayName: "Анна Петрова", roleName: "Психолог", status: "active" }]);
    setAccess.mockResolvedValue(undefined);
    const onSaved = vi.fn();
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
    const assignments = [{ userId: "user-1", displayName: "Анна Петрова", email: "anna@test.local", roleKey: "psychologist", roleName: "Психолог", grants: ["view" as const] }];
    render(<QueryClientProvider client={client}><AccessDialog studentId="student-1" assignments={assignments} onClose={vi.fn()} onSaved={onSaved} /></QueryClientProvider>);

    await screen.findByRole("button", { name: /Анна Петрова/ });
    await waitFor(() => expect(screen.getByRole("button", { name: "Сохранить" })).toBeEnabled());
    fireEvent.click(screen.getByRole("checkbox", { name: /Скачивание/ }));
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));

    await waitFor(() => expect(setAccess).toHaveBeenCalledWith("student-1", "user-1", ["view", "download"]));
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith("Разрешения обновлены"));
  });
});
