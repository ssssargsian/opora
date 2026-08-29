import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { CreateStudentDialog } from "./create-student-dialog";

const { createStudent } = vi.hoisted(() => ({ createStudent: vi.fn() }));

vi.mock("@/lib/api/students", () => ({
  studentsAPI: { create: createStudent },
}));

describe("CreateStudentDialog", () => {
  it("validates and creates a student through the API", async () => {
    createStudent.mockResolvedValue({ id: "student-1" });
    const onCreated = vi.fn();
    const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    render(<QueryClientProvider client={client}><CreateStudentDialog onClose={vi.fn()} onCreated={onCreated} /></QueryClientProvider>);

    fireEvent.click(screen.getByRole("button", { name: "Создать карточку" }));
    expect(await screen.findByText("Укажите фамилию")).toBeInTheDocument();
    expect(screen.getByText("Укажите имя")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(/Фамилия/), { target: { value: "Иванов" } });
    fireEvent.change(screen.getByLabelText(/^Имя/), { target: { value: "Иван" } });
    fireEvent.change(screen.getByLabelText("Класс"), { target: { value: "7А" } });
    fireEvent.click(screen.getByRole("button", { name: "Создать карточку" }));

    await waitFor(() => expect(createStudent).toHaveBeenCalled());
    expect(createStudent.mock.calls[0]?.[0]).toEqual(expect.objectContaining({ lastName: "Иванов", firstName: "Иван", className: "7А" }));
    await waitFor(() => expect(onCreated).toHaveBeenCalledOnce());
  });
});
