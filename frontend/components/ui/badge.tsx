import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "clay-chip inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-bold font-display",
  {
    variants: {
      variant: {
        default: "bg-grape-soft text-grape-ink",
        secondary: "bg-secondary text-secondary-foreground",
        destructive: "bg-coral-soft text-coral-ink",
        success: "bg-mint-soft text-mint-ink",
        warning: "bg-sunny-soft text-sunny-ink",
        info: "bg-sky-soft text-sky-ink",
        accent: "bg-tangerine-soft text-tangerine-ink",
        outline: "!shadow-none ring-2 ring-inset ring-border text-foreground",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
);

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, ...props }: BadgeProps) {
  return (
    <div className={cn(badgeVariants({ variant }), className)} {...props} />
  );
}

export { Badge, badgeVariants };
