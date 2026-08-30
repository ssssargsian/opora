import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { EditUserDialog } from "./edit-user-dialog";

const { roles, update } = vi.hoisted(() => ({ roles: vi.fn(), update: vi.fn() }));
vi.mock("@/lib/api/users", () => ({ usersAPI: { roles, update } }));

describe("EditUserDialog", () => {
  it("updates the selected specialist with an existing RBAC role", async () => {
    roles.mockResolvedValue([{ id: "role-1", key: "psychologist", name: "Психолог" }, { id: "role-2", key: "specialist", name: "Специалист" }]);
    update.mockResolvedValue({ id: "user-1" });
    const onUpdated = vi.fn();
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
    render(<QueryClientProvider client={client}><EditUserDialog user={{ id: "user-1", roleId: "role-1", displayName: "Петрова Анна", lastName: "Петрова", firstName: "Анна", email: "anna@test.local", roleKey: "psychologist", roleName: "Психолог", status: "active", createdAt: "2026-08-30T00:00:00Z" }} onClose={vi.fn()} onUpdated={onUpdated} /></QueryClientProvider>);

    await screen.findByRole("option", { name: "Специалист" });
    fireEvent.change(screen.getByLabelText(/^Имя/), { target: { value: "Анна-Мария" } });
    fireEvent.change(screen.getByLabelText(/^Роль/), { target: { value: "specialist" } });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));

    await waitFor(() => expect(update).toHaveBeenCalledWith("user-1", expect.objectContaining({ firstName: "Анна-Мария", roleKey: "specialist" })));
    await waitFor(() => expect(onUpdated).toHaveBeenCalled());
  });
});
