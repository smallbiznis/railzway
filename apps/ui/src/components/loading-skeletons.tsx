import * as React from "react"

import { cn } from "@/lib/utils"

type SkeletonVariant = "shimmer" | "pulse"

type RoundedSize = "none" | "sm" | "md" | "lg" | "xl" | "2xl" | "full"

const roundedClasses: Record<RoundedSize, string> = {
  none: "rounded-none",
  sm: "rounded-sm",
  md: "rounded-md",
  lg: "rounded-lg",
  xl: "rounded-xl",
  "2xl": "rounded-2xl",
  full: "rounded-full",
}

const shimmerAnimationClass =
  "animate-skeleton-shimmer motion-reduce:animate-none"
const pulseAnimationClass = "animate-pulse motion-reduce:animate-none"

const baseSkeletonClasses =
  "relative overflow-hidden bg-bg-surface-strong/70"

const shimmerGradientClasses =
  "bg-gradient-to-r from-bg-surface/70 via-bg-subtle/70 to-bg-surface/70 bg-[length:200%_100%]"

const defaultTextWidths = ["w-[70%]", "w-[90%]", "w-[50%]"]

const defaultRowCount = 6
const defaultFieldCount = 4
const defaultSidebarItemCount = 6
const defaultAvatarSize = 36
const defaultStatCardCount = 3

// Wider leading columns keep data-heavy tables readable while loading.
const defaultColumnTemplate =
  "grid-cols-[2.2fr_1.6fr_1fr_1fr_1fr_auto]"
const defaultColumnTemplateNoActions =
  "grid-cols-[2.2fr_1.6fr_1fr_1fr_1fr]"

const headerWidthsWithActions = ["w-28", "w-24", "w-20", "w-20", "w-16", "w-6"]
const headerWidthsNoActions = ["w-28", "w-24", "w-20", "w-20", "w-16"]
const cellWidthsWithActions = [
  "w-[80%]",
  "w-[70%]",
  "w-[60%]",
  "w-[60%]",
  "w-[60%]",
  "w-3",
]
const cellWidthsNoActions = ["w-[80%]", "w-[70%]", "w-[60%]", "w-[60%]", "w-[60%]"]

const toCssSize = (value?: number | string) => {
  if (value === undefined) return undefined
  return typeof value === "number" ? `${value}px` : value
}

type SkeletonBlockProps = Omit<React.ComponentProps<"div">, "width" | "height"> & {
  width?: number | string
  height?: number | string
  rounded?: RoundedSize
  variant?: SkeletonVariant
}

function SkeletonBlock({
  width,
  height,
  rounded = "md",
  variant = "shimmer",
  className,
  style,
  "aria-hidden": ariaHidden,
  ...props
}: SkeletonBlockProps) {
  const sizingStyle = {
    ...style,
    width: toCssSize(width) ?? style?.width,
    height: toCssSize(height) ?? style?.height,
  }

  return (
    <div
      aria-hidden={ariaHidden ?? true}
      data-slot="skeleton-block"
      className={cn(
        baseSkeletonClasses,
        roundedClasses[rounded],
        variant === "shimmer"
          ? cn(shimmerGradientClasses, shimmerAnimationClass)
          : pulseAnimationClass,
        className
      )}
      style={sizingStyle}
      {...props}
    />
  )
}

type SkeletonTextProps = React.ComponentProps<"div"> & {
  lines?: number
  lineHeightClass?: string
  widths?: string[]
  variant?: SkeletonVariant
  lineClassName?: string
}

function SkeletonText({
  lines = defaultTextWidths.length,
  lineHeightClass = "h-4",
  widths = defaultTextWidths,
  variant = "shimmer",
  lineClassName,
  className,
  "aria-hidden": ariaHidden,
  ...props
}: SkeletonTextProps) {
  const lineCount = Math.max(1, lines)

  return (
    <div
      aria-hidden={ariaHidden ?? true}
      data-slot="skeleton-text"
      className={cn("space-y-2", className)}
      {...props}
    >
      {Array.from({ length: lineCount }).map((_, index) => (
        <SkeletonBlock
          key={`line-${index}`}
          variant={variant}
          className={cn(lineHeightClass, widths[index % widths.length], lineClassName)}
        />
      ))}
    </div>
  )
}

type SkeletonAvatarProps = React.ComponentProps<"div"> & {
  size?: number
  variant?: SkeletonVariant
}

function SkeletonAvatar({
  size = defaultAvatarSize,
  variant = "shimmer",
  className,
  ...props
}: SkeletonAvatarProps) {
  return (
    <SkeletonBlock
      width={size}
      height={size}
      rounded="full"
      variant={variant}
      className={className}
      {...props}
    />
  )
}

type PageHeaderSkeletonProps = React.ComponentProps<"div"> & {
  showActions?: boolean
  variant?: SkeletonVariant
}

function PageHeaderSkeleton({
  showActions = true,
  variant = "shimmer",
  className,
  ...props
}: PageHeaderSkeletonProps) {
  return (
    <div
      aria-busy="true"
      data-slot="page-header-skeleton"
      className={cn("flex flex-wrap items-start justify-between gap-4", className)}
      {...props}
    >
      <div className="space-y-2">
        <SkeletonBlock variant={variant} className="h-7 w-56 md:w-72" />
        <SkeletonBlock variant={variant} className="h-4 w-64 md:w-80" />
      </div>
      {showActions && (
        <div className="flex items-center gap-2">
          <SkeletonBlock
            variant={variant}
            className="h-9 w-28 rounded-lg"
          />
        </div>
      )}
    </div>
  )
}

type StatCardSkeletonProps = React.ComponentProps<"div"> & {
  variant?: SkeletonVariant
}

function StatCardSkeleton({
  variant = "shimmer",
  className,
  ...props
}: StatCardSkeletonProps) {
  return (
    <div
      data-slot="stat-card-skeleton"
      className={cn(
        "rounded-xl border border-border-subtle bg-bg-surface p-4",
        className
      )}
      {...props}
    >
      <SkeletonBlock variant={variant} className="h-3 w-16" />
      <SkeletonBlock variant={variant} className="mt-3 h-7 w-20" />
    </div>
  )
}

type DashboardStatsSkeletonProps = React.ComponentProps<"div"> & {
  cards?: number
  variant?: SkeletonVariant
}

function DashboardStatsSkeleton({
  cards = defaultStatCardCount,
  variant = "shimmer",
  className,
  ...props
}: DashboardStatsSkeletonProps) {
  return (
    <div
      data-slot="dashboard-stats-skeleton"
      className={cn("grid gap-3 md:grid-cols-3", className)}
      {...props}
    >
      {Array.from({ length: cards }).map((_, index) => (
        <StatCardSkeleton key={`stat-${index}`} variant={variant} />
      ))}
    </div>
  )
}

type TableSkeletonProps = React.ComponentProps<"div"> & {
  rows?: number
  showActions?: boolean
  variant?: SkeletonVariant
  columnTemplate?: string
  headerWidths?: string[]
  cellWidths?: string[]
}

function TableSkeleton({
  rows = defaultRowCount,
  showActions = true,
  variant = "shimmer",
  columnTemplate,
  headerWidths: headerWidthsProp,
  cellWidths: cellWidthsProp,
  className,
  ...props
}: TableSkeletonProps) {
  const headerWidths =
    headerWidthsProp ??
    (showActions ? headerWidthsWithActions : headerWidthsNoActions)
  const cellWidths =
    cellWidthsProp ?? (showActions ? cellWidthsWithActions : cellWidthsNoActions)
  const gridTemplate =
    columnTemplate ?? (showActions ? defaultColumnTemplate : defaultColumnTemplateNoActions)
  const rowCount = Math.max(1, rows)

  return (
    <div
      aria-busy="true"
      data-slot="table-skeleton"
      className={cn(
        "rounded-lg border border-border-subtle bg-bg-surface",
        className
      )}
      {...props}
    >
      <div className="border-b border-border-subtle px-4 py-3">
        <div className={cn("grid items-center gap-4", gridTemplate)}>
          {headerWidths.map((widthClass, index) => (
            <SkeletonBlock
              key={`header-${index}`}
              variant={variant}
              className={cn("h-3", widthClass)}
            />
          ))}
        </div>
      </div>
      <div className="divide-y">
        {Array.from({ length: rowCount }).map((_, rowIndex) => (
          <div key={`row-${rowIndex}`} className="px-4 py-3">
            <div className={cn("grid items-center gap-4", gridTemplate)}>
              {cellWidths.map((widthClass, cellIndex) => {
                const isActionCell =
                  showActions && cellIndex === cellWidths.length - 1
                return (
                  <SkeletonBlock
                    key={`cell-${rowIndex}-${cellIndex}`}
                    variant={variant}
                    rounded={isActionCell ? "full" : "md"}
                    className={cn(
                      isActionCell ? "h-3 w-3 justify-self-end" : "h-4",
                      widthClass
                    )}
                  />
                )
              })}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

type FormSkeletonProps = React.ComponentProps<"div"> & {
  fields?: number
  variant?: SkeletonVariant
  showSubmit?: boolean
}

function FormSkeleton({
  fields = defaultFieldCount,
  variant = "shimmer",
  showSubmit = true,
  className,
  ...props
}: FormSkeletonProps) {
  const fieldCount = Math.max(1, fields)

  return (
    <div
      aria-busy="true"
      data-slot="form-skeleton"
      className={cn("space-y-5", className)}
      {...props}
    >
      {Array.from({ length: fieldCount }).map((_, index) => (
        <div key={`field-${index}`} className="space-y-2">
          <SkeletonBlock variant={variant} className="h-3 w-24" />
          <SkeletonBlock variant={variant} className="h-10 w-full rounded-lg" />
        </div>
      ))}
      {showSubmit && (
        <div className="flex justify-end">
          <SkeletonBlock variant={variant} className="h-10 w-28 rounded-lg" />
        </div>
      )}
    </div>
  )
}

type SidebarSkeletonProps = React.ComponentProps<"div"> & {
  items?: number
  variant?: SkeletonVariant
}

function SidebarSkeleton({
  items = defaultSidebarItemCount,
  variant = "shimmer",
  className,
  ...props
}: SidebarSkeletonProps) {
  const itemCount = Math.max(1, items)

  return (
    <div
      aria-busy="true"
      data-slot="sidebar-skeleton"
      className={cn("space-y-2", className)}
      {...props}
    >
      {Array.from({ length: itemCount }).map((_, index) => (
        <div key={`sidebar-item-${index}`} className="flex items-center gap-3">
          <SkeletonBlock variant={variant} className="h-4 w-4 rounded-sm" />
          <SkeletonBlock variant={variant} className="h-3 w-24" />
        </div>
      ))}
    </div>
  )
}

function ProductsTableSkeleton({ className }: { className?: string }) {
  return (
    <div aria-busy="true" className={cn("space-y-6", className)}>
      <PageHeaderSkeleton />
      <div className="flex flex-wrap items-center gap-3">
        <SkeletonBlock className="h-9 w-full max-w-md rounded-lg" />
      </div>
      <DashboardStatsSkeleton />
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <SkeletonBlock className="h-8 w-28 rounded-lg" />
          <SkeletonBlock className="h-8 w-24 rounded-lg" />
          <SkeletonBlock className="h-8 w-20 rounded-lg" />
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <SkeletonBlock className="h-8 w-28 rounded-lg" />
          <SkeletonBlock className="h-8 w-24 rounded-lg" />
        </div>
      </div>
      <TableSkeleton />
    </div>
  )
}

function CustomersTableSkeleton({ className }: { className?: string }) {
  return (
    <div aria-busy="true" className={cn("space-y-6", className)}>
      <PageHeaderSkeleton />
      <div className="flex flex-wrap items-center gap-3">
        <SkeletonBlock className="h-9 w-full max-w-sm rounded-lg" />
        <SkeletonBlock className="h-9 w-24 rounded-lg" />
      </div>
      <TableSkeleton
        columnTemplate="grid-cols-[2fr_2fr_1fr_auto]"
        headerWidths={["w-24", "w-28", "w-20", "w-6"]}
        cellWidths={["w-[70%]", "w-[75%]", "w-[60%]", "w-3"]}
      />
    </div>
  )
}

function ReportsPageSkeleton({ className }: { className?: string }) {
  return (
    <div aria-busy="true" className={cn("space-y-6", className)}>
      <PageHeaderSkeleton />
      <DashboardStatsSkeleton />
      <div className="grid gap-4 lg:grid-cols-[2fr_1fr]">
        <TableSkeleton rows={5} showActions={false} />
        <div className="rounded-lg border border-border-subtle bg-bg-surface p-4">
          <SkeletonText lines={4} />
          <SkeletonBlock className="mt-6 h-40 w-full rounded-lg" />
        </div>
      </div>
    </div>
  )
}

function SettingsFormSkeleton({ className }: { className?: string }) {
  return (
    <div aria-busy="true" className={cn("space-y-6", className)}>
      <PageHeaderSkeleton showActions={false} />
      <FormSkeleton fields={5} />
    </div>
  )
}

export {
  SkeletonBlock,
  SkeletonText,
  SkeletonAvatar,
  PageHeaderSkeleton,
  StatCardSkeleton,
  DashboardStatsSkeleton,
  TableSkeleton,
  FormSkeleton,
  SidebarSkeleton,
  ProductsTableSkeleton,
  CustomersTableSkeleton,
  ReportsPageSkeleton,
  SettingsFormSkeleton,
}
