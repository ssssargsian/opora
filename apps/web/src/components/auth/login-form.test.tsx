import { fireEvent, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";

import { LoginForm } from "./login-form";

const push = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));

describe("LoginForm", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
    push.mockReset();
  });

  const renderForm = () => render(<QueryClientProvider client={new QueryClient()}><LoginForm /></QueryClientProvider>);

  it("does not submit empty credentials and exposes generic validation", async () => {
    renderForm();
    fireEvent.click(screen.getByRole("button", { name: "Войти" }));
    expect(await screen.findByText("Введите корректный email")).toBeInTheDocument();
    expect(await screen.findByText("Введите пароль")).toBeInTheDocument();
  });

  it("uses browser password autocomplete semantics", () => {
    renderForm();
    expect(screen.getByLabelText("Пароль")).toHaveAttribute("autocomplete", "current-password");
  });

  it("shows a non-enumerating error for incorrect credentials", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { code: "invalid_credentials", message: "Invalid email or password" } }), { status: 401, headers: { "Content-Type": "application/json" } })));
    renderForm();
    fireEvent.change(screen.getByLabelText("Рабочая почта"), { target: { value: "person@example.com" } });
    fireEvent.change(screen.getByLabelText("Пароль"), { target: { value: "wrong-password" } });
    fireEvent.click(screen.getByRole("button", { name: "Войти" }));
    expect(await screen.findByText("Неверная почта или пароль")).toBeInTheDocument();
    expect(push).not.toHaveBeenCalled();
  });
});
