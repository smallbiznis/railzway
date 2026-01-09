import * as React from "react"
import { Slot } from "@radix-ui/react-slot"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-all disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg:not([class*='size-'])]:size-4 shrink-0 [&_svg]:shrink-0 outline-none focus-visible:border-accent-primary focus-visible:ring-accent-primary/50 focus-visible:ring-[3px] aria-invalid:ring-status-error/25 aria-invalid:border-status-error",
  {
    variants: {
      variant: {
        default: "bg-accent-primary text-text-inverse hover:bg-accent-primary/90",
        destructive:
          "bg-status-error text-text-inverse hover:bg-status-error/90 focus-visible:ring-status-error/40",
        outline:
          "border border-border-strong bg-bg-primary shadow-xs text-text-secondary hover:bg-accent-primary/10 hover:text-accent-primary",
        secondary:
          "bg-bg-surface-strong text-text-primary hover:bg-bg-surface-strong/90",
        ghost:
          "hover:bg-bg-surface-strong hover:text-accent-primary",
        link: "text-accent-primary underline-offset-4 hover:underline",
      },
      size: {
        default: "h-9 px-4 py-2 has-[>svg]:px-3",
        sm: "h-8 rounded-md gap-1.5 px-3 has-[>svg]:px-2.5",
        lg: "h-10 rounded-md px-6 has-[>svg]:px-4",
        icon: "size-9",
        "icon-sm": "size-8",
        "icon-lg": "size-10",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

function Button({
  className,
  variant = "default",
  size = "default",
  asChild = false,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean
  }) {
  const Comp = asChild ? Slot : "button"

  return (
    <Comp
      data-slot="button"
      data-variant={variant}
      data-size={size}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Button, buttonVariants }
