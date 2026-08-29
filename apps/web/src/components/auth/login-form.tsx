"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useRouter } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { z } from "zod";

import { Button } from "@/components/ui/button";
import { APIError, authAPI } from "@/lib/api/client";

const schema = z.object({
  email: z.email("Введите корректный email"),
  password: z.string().min(1, "Введите пароль"),
});

type LoginValues = z.infer<typeof schema>;

export function LoginForm() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { register, handleSubmit, setError, formState: { errors, isSubmitting } } = useForm<LoginValues>({
    resolver: zodResolver(schema),
    defaultValues: { email: "", password: "" },
  });

  const onSubmit = async (values: LoginValues) => {
    try {
      const user = await authAPI.login(values.email, values.password);
      queryClient.setQueryData(["me"], user);
      router.replace("/students");
    } catch (error) {
      if (error instanceof APIError && error.status === 429) setError("root", { message: "Слишком много попыток. Попробуйте позднее" });
      else if (error instanceof APIError && error.status === 0) setError("root", { message: "Сервер временно недоступен" });
      else setError("root", { message: "Неверная почта или пароль" });
    }
  };

  return (
    <form className="login-form" onSubmit={handleSubmit(onSubmit)} noValidate>
      <label>
        <span>Рабочая почта</span>
        <input type="email" autoComplete="username" {...register("email")} aria-invalid={Boolean(errors.email)} />
        {errors.email && <small role="alert">{errors.email.message}</small>}
      </label>
      <label>
        <span>Пароль</span>
        <input type="password" autoComplete="current-password" {...register("password")} aria-invalid={Boolean(errors.password)} />
        {errors.password && <small role="alert">{errors.password.message}</small>}
      </label>
      {errors.root && <div className="form-error" role="alert">{errors.root.message}</div>}
      <Button type="submit" disabled={isSubmitting}>{isSubmitting ? "Проверяем данные…" : "Войти"}</Button>
    </form>
  );
}
