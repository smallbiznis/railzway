import * as React from "react"
import { Slot } from "@radix-ui/react-slot"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

const badgeVariants = cva(
  "inline-flex items-center justify-center rounded-full border border-border-subtle px-2 py-0.5 text-xs font-medium w-fit whitespace-nowrap shrink-0 [&>svg]:size-3 gap-1 [&>svg]:pointer-events-none focus-visible:border-accent-primary focus-visible:ring-accent-primary/50 focus-visible:ring-[3px] aria-invalid:ring-status-error/25 aria-invalid:border-status-error transition-[color,box-shadow] overflow-hidden",
  {
    variants: {
      variant: {
        default:
          "border-transparent bg-accent-primary text-text-inverse [a&]:hover:bg-accent-primary/90",
        secondary:
          "border-transparent bg-bg-surface-strong text-text-primary [a&]:hover:bg-bg-surface-strong/90",
        destructive:
          "border-transparent bg-status-error text-text-inverse [a&]:hover:bg-status-error/90 focus-visible:ring-status-error/40",
        outline:
          "text-text-secondary [a&]:hover:bg-bg-surface-strong [a&]:hover:text-text-primary",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

function Badge({
  className,
  variant,
  asChild = false,
  ...props
}: React.ComponentProps<"span"> &
  VariantProps<typeof badgeVariants> & { asChild?: boolean }) {
  const Comp = asChild ? Slot : "span"

  return (
    <Comp
      data-slot="badge"
      className={cn(badgeVariants({ variant }), className)}
      {...props}
    />
  )
}

export { Badge, badgeVariants }
