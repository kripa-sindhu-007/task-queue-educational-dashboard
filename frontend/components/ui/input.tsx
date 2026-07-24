import * as React from "react";
import { cn } from "@/lib/utils";

const Input = React.forwardRef<
  HTMLInputElement,
  React.InputHTMLAttributes<HTMLInputElement>
>(({ className, type, ...props }, ref) => (
  <input
    type={type}
    className={cn(
      "clay-inset flex h-10 w-full px-3.5 py-1 text-sm font-medium text-foreground placeholder:text-muted-foreground transition-shadow focus-visible:outline-none focus-visible:shadow-[inset_0_3px_8px_rgba(79,70,180,0.20),0_0_0_4px_rgba(99,102,241,0.30)] disabled:cursor-not-allowed disabled:opacity-50",
      className
    )}
    ref={ref}
    {...props}
  />
));
Input.displayName = "Input";

export { Input };
