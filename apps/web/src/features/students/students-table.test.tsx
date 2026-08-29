import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { StudentsTable } from "./students-table";

vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: React.ComponentProps<"a">) => <a href={href} {...props}>{children}</a>,
}));

const students = [
  { id: "student-1", fullName: "Иванов Иван Иванович", className: "7А", documentCount: 4, updatedAt: "29.08.2026" },
  { id: "student-2", fullName: "Соколова Алиса", className: "5Б", documentCount: 2, updatedAt: "28.08.2026" },
];

describe("StudentsTable", () => {
  it("filters by name without losing the student link", () => {
    render(<StudentsTable students={students} />);
    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "Иванов" } });
    expect(screen.getByRole("link", { name: "Иванов Иван Иванович" })).toHaveAttribute("href", "/students/student-1");
    expect(screen.queryByText("Соколова Алиса")).not.toBeInTheDocument();
  });
});
