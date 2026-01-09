export type InvoiceTemplate = {
  id: string
  name: string
  memo: string
  footer: string
  updatedAt: string
  status: "ACTIVE" | "DRAFT"
  isDefault?: boolean
  customFields?: Array<{ label: string; value: string }>
  lineItemGrouping?: string
}

export const invoiceTemplates: InvoiceTemplate[] = [
  {
    id: "default",
    name: "Default invoice",
    memo: "Have questions? Call us at 555-123-4567",
    footer: "Thank you for your business!",
    updatedAt: "2026-01-24T13:00:00Z",
    status: "ACTIVE",
    isDefault: true,
    customFields: [],
    lineItemGrouping: "None",
  },
  {
    id: "modern",
    name: "Modern minimal",
    memo: "Email billing@smallbiznis.co with any questions.",
    footer: "Payable within 30 days. Late fees apply after 45 days.",
    updatedAt: "2026-01-22T09:30:00Z",
    status: "DRAFT",
    customFields: [
      { label: "PO number", value: "PO-1029" },
      { label: "Project", value: "Atlas rollout" },
    ],
    lineItemGrouping: "Service category",
  },
]

export const findInvoiceTemplate = (templateId?: string) => {
  if (!templateId) return null
  return invoiceTemplates.find((template) => template.id === templateId) ?? null
}
