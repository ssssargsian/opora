import { cn } from "@/lib/cn";

export function Logo({ className, withWordmark = true }: { className?: string; withWordmark?: boolean }) {
  return (
    <span className={cn("opora-logo", className)} aria-label="Опора">
      <svg className="opora-symbol" viewBox="0 0 40 40" role="img" aria-hidden="true">
        <path d="M20 3.75 34 11.5v10.2c0 7.15-5.17 12.26-14 14.55C11.17 33.96 6 28.85 6 21.7V11.5L20 3.75Z" fill="currentColor" opacity=".16" />
        <path d="M20 6.75 31 12.8v8.7c0 5.45-3.73 9.55-11 11.7-7.27-2.15-11-6.25-11-11.7v-8.7L20 6.75Z" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinejoin="round" />
        <circle cx="20" cy="15" r="3.25" fill="currentColor" />
        <path d="M13.5 26.4c1.25-4 3.5-6 6.5-6s5.25 2 6.5 6" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" />
        <path d="M12.25 18.8 9 21.05m18.75-2.25L31 21.05" stroke="currentColor" strokeWidth="2.15" strokeLinecap="round" />
      </svg>
      {withWordmark && <span className="opora-wordmark"><strong>Опора</strong><small>Школьная служба</small></span>}
    </span>
  );
}
