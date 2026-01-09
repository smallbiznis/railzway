type Props = {
  title: string
  description?: string
}

export function PlaceholderPage({ title, description }: Props) {
  return (
    <div className="space-y-2">
      <h1 className="text-2xl font-semibold">{title}</h1>
      {description && <p className="text-text-muted">{description}</p>}
    </div>
  )
}
