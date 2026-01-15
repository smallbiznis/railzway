import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { AnimatePresence, motion } from 'motion/react'
import { loadStripe } from '@stripe/stripe-js'
import {
  Elements,
  PaymentElement,
  useElements,
  useStripe,
} from '@stripe/react-stripe-js'
import {
  Drawer,
  DrawerClose,
  DrawerContent,
  DrawerHeader,
  DrawerTrigger,
} from '@/components/ui/drawer'
import { useIntl, type IntlShape } from 'react-intl'

import { AdyenCheckout } from './AdyenCheckout'
import { BraintreeCheckout } from './BraintreeCheckout'

type FlowState =
  | 'invoice'
  | 'payment_summary'
  | 'select_method'
  | 'payment_form' // Renamed from card_form
  | 'processing'
  | 'success'
  | 'failed'

type InvoiceStatus = 'unpaid' | 'processing' | 'paid' | 'failed'

type PaymentMethodType = 'card' | 'bank_transfer' | 'local_payment'

type PublicPaymentMethod = {
  provider: string
  type: PaymentMethodType
  display_name: string
  supports_installment: boolean
  publishable_key?: string
}

type Invoice = {
  orgName: string
  invoiceNumber: string
  issueDate: string
  dueDate: string
  dueDateRaw?: string
  paidDate?: string
  paidDateRaw?: string
  paymentState?: string
  invoiceStatus?: string
  billToName: string
  billToEmail: string
  currency: string
  amountDue: number
  items: Array<{
    name: string
    quantity: number
    unitPrice: number
    total: number
    lineType?: string
  }>
  subtotal: number
  tax: number
  total: number
}

type CheckoutSessionResponse = {
  provider: string
  session_token: string
  public_config?: any
  metadata?: any
}

type PublicInvoiceResponse = {
  status: InvoiceStatus
  invoice: {
    org_id: string
    org_name: string
    invoice_number: string
    issue_date: string
    due_date: string
    paid_date?: string
    payment_state?: string
    invoice_status?: string
    bill_to_name: string
    bill_to_email: string
    currency: string
    amount_due: number
    subtotal_amount: number
    tax_amount: number
    total_amount: number
    items: Array<{
      description: string
      quantity: number
      unit_price: number
      amount: number
      line_type?: string
    }>
  }
}

type PublicPaymentMethodsResponse = {
  methods: PublicPaymentMethod[]
}

type PublicInvoiceErrorResponse = {
  code?: string
  message?: string
}

type StripePromise = ReturnType<typeof loadStripe>

const DEFAULT_STRIPE_PUBLISHABLE_KEY = import.meta.env.VITE_STRIPE_PUBLISHABLE_KEY as
  | string
  | undefined

const sampleInvoice: Invoice = {
  orgName: 'Railzway',
  invoiceNumber: 'INV-2026-00492',
  issueDate: '1 February 2026',
  dueDate: '7 February 2026',
  dueDateRaw: '2026-02-07T00:00:00Z',
  billToName: 'Juniper Market Ltd.',
  billToEmail: 'billing@junipermarket.co',
  currency: 'USD',
  amountDue: 108,
  items: [
    {
      name: 'Valora Core - Starter Plan (Feb 2026)',
      quantity: 1,
      unitPrice: 79,
      total: 79,
    },
    {
      name: 'Metered usage: API calls (24,500)',
      quantity: 1,
      unitPrice: 19,
      total: 19,
    },
    {
      name: 'Support concierge (priority responses)',
      quantity: 1,
      unitPrice: 10,
      total: 10,
    },
  ],
  subtotal: 108,
  tax: 0,
  total: 108,
  invoiceStatus: 'FINALIZED',
}

const stepVariants = {
  initial: { opacity: 0 },
  animate: { opacity: 1 },
  exit: { opacity: 0 },
}

export function InvoiceFlow({
  token,
  orgId,
}: {
  token?: string
  orgId?: string
}) {
  const intl = useIntl()
  const invoiceUnavailableTitle = intl.formatMessage({
    id: 'invoice.unavailableTitle',
    defaultMessage: 'Invoice not available',
  })
  const invoiceUnavailableMessage = intl.formatMessage({
    id: 'invoice.unavailableMessage',
    defaultMessage:
      'This invoice link is invalid, expired, or no longer available.',
  })
  const route = useMemo(() => getPublicInvoiceRoute(), [])
  const invoiceToken = token ?? route.token
  const invoiceOrgId = orgId ?? route.orgId
  const [flowState, setFlowState] = useState<FlowState>('invoice')
  const [invoice, setInvoice] = useState<Invoice>(sampleInvoice)
  const [paymentMethods, setPaymentMethods] = useState<PublicPaymentMethod[]>([])
  const [paymentMethod, setPaymentMethod] = useState<PublicPaymentMethod | null>(
    null
  )
  const [isInvoiceLoading, setIsInvoiceLoading] = useState(true)
  const [invoiceError, setInvoiceError] = useState<{
    title: string
    message: string
  } | null>(null)
  const [checkoutSession, setCheckoutSession] = useState<CheckoutSessionResponse | null>(null)
  const [isBusy, setIsBusy] = useState(false)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)

  useEffect(() => {
    if (!invoiceToken || !invoiceOrgId) {
      setInvoiceError({
        title: invoiceUnavailableTitle,
        message: invoiceUnavailableMessage,
      })
      setIsInvoiceLoading(false)
      return
    }
    void refreshInvoice()
  }, [invoiceToken, invoiceOrgId])

  useEffect(() => {
    if (flowState !== 'processing' || !invoiceToken || !invoiceOrgId) return
    const controller = new AbortController()
    const poll = async () => {
      try {
        const status = await fetchInvoiceStatus(
          invoiceOrgId,
          invoiceToken,
          controller.signal
        )
        if (!status) return
        if (status === 'paid') {
          setFlowState('success')
        } else if (status === 'failed') {
          setFlowState('failed')
        }
      } catch (error) {
        if ((error as Error).name === 'AbortError') return
      }
    }

    void poll()
    const interval = window.setInterval(poll, 3000)
    return () => {
      controller.abort()
      window.clearInterval(interval)
    }
  }, [flowState, invoiceToken, invoiceOrgId])

  async function refreshInvoice() {
    if (!invoiceOrgId || !invoiceToken) return
    setIsInvoiceLoading(true)
    try {
      const response = await fetch(
        `/public/orgs/${invoiceOrgId}/invoices/${invoiceToken}`
      )
      if (!response.ok) {
        const errorPayload = await readPublicInvoiceError(response)
        setInvoiceError({
          title: invoiceUnavailableTitle,
          message: errorPayload?.message ?? invoiceUnavailableMessage,
        })
        return
      }
      const data = (await response.json()) as PublicInvoiceResponse
      const mapped = mapInvoicePayload(data.invoice, intl)
      setInvoice(mapped)
      setInvoiceError(null)
      if (mapped.invoiceStatus?.toUpperCase() === 'VOID') {
        setFlowState('invoice')
      } else if (data.status === 'paid') {
        setFlowState(flowState === 'processing' ? 'success' : 'invoice')
      } else if (data.status === 'failed') {
        setFlowState(flowState === 'processing' ? 'failed' : 'invoice')
      } else if (data.status === 'processing' && flowState === 'processing') {
        setFlowState('processing')
      }
      void refreshPaymentMethods()
    } catch (error) {
      setInvoiceError({
        title: invoiceUnavailableTitle,
        message: invoiceUnavailableMessage,
      })
    } finally {
      setIsInvoiceLoading(false)
    }
  }

  async function refreshPaymentMethods() {
    if (!invoiceOrgId) return
    try {
      const response = await fetch(`/public/orgs/${invoiceOrgId}/payment_methods`)
      if (!response.ok) {
        return
      }
      const data = (await response.json()) as PublicPaymentMethodsResponse
      const methods = Array.isArray(data.methods) ? data.methods : []
      setPaymentMethods(methods)
      if (!paymentMethod && methods.length > 0) {
        setPaymentMethod(methods[0])
      }
    } catch (error) {
      // ignore
    }
  }

  const stripeKey = useMemo(() => {
    const stripeMethod = paymentMethods.find(
      (method) => method.provider === 'stripe' && method.type === 'card'
    )
    return stripeMethod?.publishable_key ?? DEFAULT_STRIPE_PUBLISHABLE_KEY ?? ''
  }, [paymentMethods])

  const stripePromise = useMemo<StripePromise | null>(() => {
    if (!stripeKey) return null
    return loadStripe(stripeKey)
  }, [stripeKey])

  async function createCheckoutSession(provider: string) {
    if (!invoiceToken || !invoiceOrgId) {
      throw new Error('Missing invoice token')
    }
    const response = await fetch(
      `/public/orgs/${invoiceOrgId}/invoices/${invoiceToken}/checkout-session?provider=${provider}`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      }
    )
    if (!response.ok) {
      if (response.status === 404 || response.status === 410) {
        const errorPayload = await readPublicInvoiceError(response)
        setInvoiceError({
          title: invoiceUnavailableTitle,
          message: errorPayload?.message ?? invoiceUnavailableMessage,
        })
      }
      throw new Error('Unable to start payment')
    }
    const data = (await response.json()) as CheckoutSessionResponse
    setCheckoutSession(data)
  }

  async function beginPaymentFlow(method: PublicPaymentMethod | null) {
    if (!method) {
      setErrorMessage(
        intl.formatMessage({
          id: 'payment.selectMethodHint',
          defaultMessage: 'Select a payment method to continue.',
        })
      )
      return
    }
    // Removed strict check for Stripe/Card. Now validating against supported providers.
    if (!['stripe', 'adyen', 'braintree'].includes(method.provider)) {
      setErrorMessage(
        intl.formatMessage({
          id: 'payment.methodUnavailable',
          defaultMessage: 'Selected payment method is not available yet.',
        })
      )
      return
    }

    setErrorMessage(null)
    setIsBusy(true)
    try {
      await createCheckoutSession(method.provider)
      setFlowState('payment_form')
    } catch (error) {
      setErrorMessage(
        intl.formatMessage({
          id: 'payment.startError',
          defaultMessage: 'Unable to start payment. Please try again.',
        })
      )
      setFlowState('select_method')
    } finally {
      setIsBusy(false)
    }
  }

  const handleSelectMethod = (method: PublicPaymentMethod) => {
    if (isBusy) return
    setPaymentMethod(method)
    setErrorMessage(null)
    // Removed auto-start for Stripe to keep inconsistent flow or maybe keep it?
    // Let's require explicit continue for all for consistency, or auto-start if it's the only one.
    // If user clicks a method in the list, we just select it.
    // Logic below was: if Stripe, auto start. Let's disable auto-start to avoid accidents.
  }

  async function handleChooseMethod() {
    setCheckoutSession(null)
    await beginPaymentFlow(paymentMethod)
  }

  async function handleTryAgain() {
    setCheckoutSession(null)
    await beginPaymentFlow(paymentMethod)
  }

  const handleStartPaymentSelection = () => {
    if (paymentMethods.length === 1) {
      const method = paymentMethods[0]
      handleSelectMethod(method)
      if (['stripe', 'adyen', 'braintree'].includes(method.provider)) {
        return
      }
    }
    setFlowState('select_method')
  }

  // New handler for Braintree processing
  const handleBraintreeSuccess = async (nonce: string) => {
    if (!checkoutSession || !invoiceToken || !invoiceOrgId) return
    try {
      setFlowState('processing')
      const response = await fetch(
        `/public/orgs/${invoiceOrgId}/invoices/${invoiceToken}/process-payment`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            provider: 'braintree',
            nonce
          })
        }
      )
      const data = await response.json()
      if (data.success) {
        setFlowState('success')
      } else {
        throw new Error(data.message || 'Payment failed')
      }
    } catch (e) {
      setFlowState('failed')
    }
  }

  // New handler for Adyen success (redirects or state change)
  const handleAdyenSuccess = () => {
    setFlowState('success')
  }

  return (
    <div className="min-h-screen bg-neutral-50 text-neutral-900">
      <div className="mx-auto flex min-h-screen max-w-md items-start px-4 py-8">
        <div className="w-full">
          <AnimatePresence mode="wait">
            {/* Error Screen logic same */}
            {invoiceError ? (
              <motion.div
                key="error"
                variants={stepVariants}
                initial="initial"
                animate="animate"
                exit="exit"
                transition={{ duration: 0.2 }}
              >
                <PublicInvoiceErrorScreen
                  title={invoiceError.title}
                  message={invoiceError.message}
                  onClose={() => window.history.back()}
                />
              </motion.div>
            ) : null}

            {/* Invoice Screen logic same */}
            {!invoiceError && flowState === 'invoice' ? (
              <motion.div
                key="invoice"
                variants={stepVariants}
                initial="initial"
                animate="animate"
                exit="exit"
                transition={{ duration: 0.16 }}
              >
                <InvoiceScreen
                  invoice={invoice}
                  onProceed={() => setFlowState('payment_summary')}
                  isLoading={isInvoiceLoading}
                />
              </motion.div>
            ) : null}

            {/* Payment Summary logic same */}
            {!invoiceError && flowState === 'payment_summary' ? (
              <motion.div
                key="payment_summary"
                variants={stepVariants}
                initial="initial"
                animate="animate"
                exit="exit"
                transition={{ duration: 0.16 }}
              >
                <PaymentSummaryScreen
                  invoice={invoice}
                  onChooseMethod={handleStartPaymentSelection}
                />
              </motion.div>
            ) : null}

            {/* Select Method logic same */}
            {!invoiceError && flowState === 'select_method' ? (
              <motion.div
                key="select_method"
                variants={stepVariants}
                initial="initial"
                animate="animate"
                exit="exit"
                transition={{ duration: 0.16 }}
              >
                <SelectMethodScreen
                  invoice={invoice}
                  paymentMethods={paymentMethods}
                  selectedMethod={paymentMethod}
                  onSelect={handleSelectMethod}
                  onContinue={handleChooseMethod}
                  isBusy={isBusy}
                  errorMessage={errorMessage}
                />
              </motion.div>
            ) : null}

            {/* Payment Form (Generic) */}
            {!invoiceError && flowState === 'payment_form' ? (
              <motion.div
                key="payment_form"
                variants={stepVariants}
                initial="initial"
                animate="animate"
                exit="exit"
                transition={{ duration: 0.16 }}
              >
                {/* Switch based on provider */}
                {paymentMethod?.provider === 'stripe' ? (
                  <CardFormScreen
                    invoice={invoice}
                    clientSecret={checkoutSession?.session_token ?? null}
                    isBusy={isBusy}
                    stripePromise={stripePromise}
                    onBack={() => setFlowState('select_method')}
                    onProcessing={() => {
                      setFlowState('processing')
                    }}
                  />
                ) : paymentMethod?.provider === 'adyen' && checkoutSession ? (
                  <CardShell title={invoice.orgName} subtitle="Pay with Adyen">
                    <AdyenCheckout
                      sessionId={checkoutSession.metadata?.session_id}
                      sessionData={checkoutSession.session_token}
                      environment={checkoutSession.public_config?.environment || 'test'}
                      clientKey={checkoutSession.public_config?.client_key}
                      onSuccess={handleAdyenSuccess}
                      onFailure={() => setFlowState('failed')}
                    />
                    <button onClick={() => setFlowState('select_method')} className="mt-4 w-full text-center text-sm text-neutral-500">Back</button>
                  </CardShell>
                ) : paymentMethod?.provider === 'braintree' && checkoutSession ? (
                  <CardShell title={invoice.orgName} subtitle="Pay with Braintree">
                    <BraintreeCheckout
                      authorization={checkoutSession.session_token}
                      onSuccess={handleBraintreeSuccess}
                      onFailure={() => setFlowState('failed')}
                    />
                    <button onClick={() => setFlowState('select_method')} className="mt-4 w-full text-center text-sm text-neutral-500">Back</button>
                  </CardShell>
                ) : (
                  <div>Loading payment provider...</div>
                )}
              </motion.div>
            ) : null}

            {/* Processing logic same */}
            {!invoiceError && flowState === 'processing' ? (
              <motion.div
                key="processing"
                variants={stepVariants}
                initial="initial"
                animate="animate"
                exit="exit"
                transition={{ duration: 0.16 }}
              >
                <ProcessingScreen invoice={invoice} paymentMethod={paymentMethod} />
              </motion.div>
            ) : null}

            {!invoiceError && flowState === 'success' ? (
              <motion.div
                key="success"
                variants={stepVariants}
                initial="initial"
                animate="animate"
                exit="exit"
                transition={{ duration: 0.16 }}
              >
                <SuccessScreen invoice={invoice} paymentMethod={paymentMethod} />
              </motion.div>
            ) : null}

            {!invoiceError && flowState === 'failed' ? (
              <motion.div
                key="failed"
                variants={stepVariants}
                initial="initial"
                animate="animate"
                exit="exit"
                transition={{ duration: 0.16 }}
              >
                <FailedScreen
                  onTryAgain={handleTryAgain}
                  onChooseAnother={() => setFlowState('select_method')}
                />
              </motion.div>
            ) : null}
          </AnimatePresence>

        </div>
      </div>
    </div>
  )
}

function InvoiceScreen({
  invoice,
  onProceed,
  isLoading,
}: {
  invoice: Invoice
  onProceed: () => void
  isLoading: boolean
}) {
  const intl = useIntl()
  const headerRef = useRef<HTMLDivElement | null>(null)
  const [showSticky, setShowSticky] = useState(false)
  const [copied, setCopied] = useState(false)
  const badge = useMemo(() => derivePaymentBadge(invoice, intl), [invoice, intl])

  useEffect(() => {
    const handleScroll = () => {
      if (!headerRef.current) return
      const rect = headerRef.current.getBoundingClientRect()
      setShowSticky(rect.bottom < 12)
    }
    handleScroll()
    window.addEventListener('scroll', handleScroll, { passive: true })
    window.addEventListener('resize', handleScroll)
    return () => {
      window.removeEventListener('scroll', handleScroll)
      window.removeEventListener('resize', handleScroll)
    }
  }, [])

  const canPay = badge.state === 'open' || badge.state === 'overdue'
  const ctaLabel = canPay
    ? intl.formatMessage({
      id: 'invoice.proceedToPayment',
      defaultMessage: 'Proceed to payment',
    })
    : badge.label
  const stickyVisible = showSticky && canPay && !isLoading

  const headerItem = (delay: number) => ({
    initial: { opacity: 0, y: 6 },
    animate: { opacity: 1, y: 0 },
    transition: { duration: 0.3, delay },
  })

  const handleCopyLink = async () => {
    const link = window.location.href
    try {
      await navigator.clipboard.writeText(link)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1800)
    } catch {
      setCopied(false)
    }
  }

  return (
    <div className="relative pb-24">
      <motion.div
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.35 }}
        className="grid gap-6"
      >
        <motion.section
          ref={headerRef}
          className="rounded-2xl border border-neutral-200 bg-white p-6 shadow-[0_20px_40px_-32px_rgba(15,23,42,0.35)]"
        >
          <div className="flex items-start justify-between gap-4">
            <div>
              <p className="text-lg font-semibold text-neutral-900">
                {invoice.orgName}
              </p>
              <p className="mt-1 text-xs font-semibold uppercase tracking-[0.18em] text-neutral-500">
                {intl.formatMessage({
                  id: 'invoice.title',
                  defaultMessage: 'Invoice',
                })}
              </p>
            </div>
            <span
              className={`rounded-full px-3 py-1 text-[10px] font-semibold uppercase tracking-[0.18em] ring-1 ${badge.tone === 'success'
                ? 'bg-emerald-50 text-emerald-700 ring-emerald-100'
                : badge.tone === 'danger'
                  ? 'bg-rose-50 text-rose-700 ring-rose-100'
                  : badge.tone === 'warning'
                    ? 'bg-amber-50 text-amber-700 ring-amber-100'
                    : 'bg-slate-100 text-slate-700 ring-slate-200'
                }`}
            >
              {badge.label}
            </span>
          </div>

          {isLoading ? (
            <motion.div
              className="mt-6 grid gap-4"
              animate={{ opacity: [0.45, 0.8, 0.45] }}
              transition={{ duration: 1.6, repeat: Infinity }}
            >
              <div className="h-5 w-32 rounded-full bg-neutral-200" />
              <div className="h-10 w-52 rounded-xl bg-neutral-200" />
              <div className="h-4 w-36 rounded-full bg-neutral-200" />
              <div className="h-12 w-full rounded-xl bg-neutral-200" />
            </motion.div>
          ) : (
            <>
              <motion.div {...headerItem(0.04)} className="mt-6">
                <div className="text-xs font-semibold uppercase tracking-[0.2em] text-neutral-500">
                  {intl.formatMessage({
                    id: 'invoice.totalDue',
                    defaultMessage: 'Total due',
                  })}
                </div>
                <div className="mt-2 text-4xl font-semibold text-neutral-900">
                  {formatMoney(intl, invoice.currency, invoice.total)}
                </div>
                <div className="mt-2 text-sm text-neutral-600">
                  {intl.formatMessage(
                    { id: 'invoice.dueOn', defaultMessage: 'Due on {date}' },
                    { date: invoice.dueDate }
                  )}
                </div>
              </motion.div>

              <motion.button
                {...headerItem(0.06)}
                className="mt-2 flex items-center gap-1 text-sm font-medium text-neutral-500 hover:text-neutral-900"
                onClick={() => {
                  const trigger = document.querySelector('[data-drawer-trigger]') as HTMLButtonElement | null
                  trigger?.click()
                }}
              >
                {intl.formatMessage({
                  id: 'invoice.viewDetails',
                  defaultMessage: 'View invoice details',
                })}
                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="m9 18 6-6-6-6" />
                </svg>
              </motion.button>

              {badge.state === 'void' ? (
                <motion.div
                  {...headerItem(0.08)}
                  className="mt-4 rounded-xl border border-rose-100 bg-rose-50 px-4 py-3 text-sm text-rose-700"
                >
                  {intl.formatMessage({
                    id: 'invoice.voidedMessage',
                    defaultMessage:
                      'This invoice has been voided and is no longer payable.',
                  })}
                </motion.div>
              ) : badge.state === 'paid' ? (
                <motion.div
                  {...headerItem(0.08)}
                  className="mt-4 rounded-xl border border-emerald-100 bg-emerald-50 px-4 py-3 text-sm text-emerald-700"
                >
                  {invoice.paidDate
                    ? intl.formatMessage(
                      {
                        id: 'invoice.paidOn',
                        defaultMessage: 'Paid on {date}.',
                      },
                      { date: invoice.paidDate }
                    )
                    : intl.formatMessage({
                      id: 'invoice.paidMessage',
                      defaultMessage: 'This invoice has been paid.',
                    })}
                </motion.div>
              ) : null}

              <motion.div
                {...headerItem(0.1)}
                className="mt-5 grid gap-2 text-sm text-neutral-600"
              >
                <KeyValue
                  label={intl.formatMessage({
                    id: 'invoice.number',
                    defaultMessage: 'Invoice number',
                  })}
                  value={invoice.invoiceNumber}
                />
                <KeyValue
                  label={intl.formatMessage({
                    id: 'invoice.issueDate',
                    defaultMessage: 'Issue date',
                  })}
                  value={invoice.issueDate}
                />
                <KeyValue
                  label={intl.formatMessage({
                    id: 'invoice.dueDate',
                    defaultMessage: 'Due date',
                  })}
                  value={invoice.dueDate}
                />
              </motion.div>

              <motion.div {...headerItem(0.16)} className="mt-6 grid gap-3">
                <motion.button
                  className="w-full rounded-xl bg-blue-600 py-3 text-sm font-semibold text-white shadow-sm disabled:cursor-not-allowed disabled:bg-neutral-200 disabled:text-neutral-500"
                  type="button"
                  onClick={onProceed}
                  disabled={!canPay}
                  whileHover={
                    canPay
                      ? {
                        scale: 1.02,
                        boxShadow: '0 12px 28px -18px rgba(37, 99, 235, 0.8)',
                      }
                      : undefined
                  }
                  whileTap={canPay ? { scale: 0.98 } : undefined}
                >
                  {ctaLabel}
                </motion.button>

                <div className="grid gap-2 sm:grid-cols-2">
                  <button
                    className="w-full rounded-xl border border-neutral-200 bg-white py-3 text-sm font-semibold text-neutral-700"
                    type="button"
                    onClick={() => window.print()}
                    aria-label={intl.formatMessage({
                      id: 'invoice.downloadPDF',
                      defaultMessage: 'Download PDF',
                    })}
                  >
                    {intl.formatMessage({
                      id: 'invoice.downloadPDF',
                      defaultMessage: 'Download PDF',
                    })}
                  </button>
                  <button
                    className="w-full rounded-xl border border-neutral-200 bg-white py-3 text-sm font-semibold text-neutral-700"
                    type="button"
                    onClick={handleCopyLink}
                    aria-label={intl.formatMessage({
                      id: 'invoice.copyLink',
                      defaultMessage: 'Copy invoice link',
                    })}
                  >
                    {copied
                      ? intl.formatMessage({
                        id: 'invoice.copiedLink',
                        defaultMessage: 'Copied link',
                      })
                      : intl.formatMessage({
                        id: 'invoice.copyLink',
                        defaultMessage: 'Copy invoice link',
                      })}
                  </button>
                </div>
              </motion.div>
            </>
          )}
        </motion.section>
        <section className="flex justify-center">
          {isLoading ? (
            <div className="h-4 w-32 animate-pulse rounded-full bg-neutral-200" />
          ) : (
            <Drawer direction="right">
              <DrawerTrigger asChild>
                <button
                  data-drawer-trigger
                  className="hidden"
                  type="button"
                >
                  Trigger
                </button>
              </DrawerTrigger>
              <DrawerContent className="flex flex-col h-full rounded-none sm:max-w-md">
                {/* Sticky Header */}
                <DrawerHeader className="flex-none border-b border-neutral-100 px-6 py-4 text-left">
                  <DrawerClose asChild>
                    <button className="group flex items-center gap-2 text-sm font-medium text-neutral-500 transition-colors hover:text-neutral-900">
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="20"
                        height="20"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        className="opacity-70 group-hover:opacity-100"
                      >
                        <path d="M18 6 6 18" />
                        <path d="m6 6 12 12" />
                      </svg>
                      {intl.formatMessage({
                        id: 'invoice.closeDetails',
                        defaultMessage: 'Close invoice details',
                      })}
                    </button>
                  </DrawerClose>
                </DrawerHeader>

                {/* Scrollable Content */}
                <div className="flex-1 overflow-y-auto p-6">
                  <h2 className="mb-6 text-2xl font-bold text-neutral-900">
                    {invoice.paidDate ? (
                      intl.formatMessage({
                        id: 'invoice.paidTitle',
                        defaultMessage: 'Paid on {date}',
                      }, { date: invoice.paidDate })
                    ) : (
                      intl.formatMessage({
                        id: 'invoice.dueTitle',
                        defaultMessage: 'Due by {date}',
                      }, { date: invoice.dueDate })
                    )}
                  </h2>

                  <div className="grid gap-6">
                    <section className="grid gap-4 text-sm">
                      <h3 className="text-xs font-semibold uppercase tracking-wider text-neutral-500">
                        {intl.formatMessage({ id: 'invoice.summary', defaultMessage: 'Summary' })}
                      </h3>

                      <div className="grid grid-cols-[100px_1fr] gap-y-3">
                        <span className="text-neutral-500">
                          {intl.formatMessage({ id: 'invoice.to', defaultMessage: 'To' })}
                        </span>
                        <span className="font-medium text-neutral-900">{invoice.billToName}</span>

                        <span className="text-neutral-500">
                          {intl.formatMessage({ id: 'invoice.from', defaultMessage: 'From' })}
                        </span>
                        <span className="text-neutral-900">{invoice.orgName}</span>

                        <span className="text-neutral-500">
                          {intl.formatMessage({ id: 'invoice.invoiceNo', defaultMessage: 'Invoice' })}
                        </span>
                        <span className="text-neutral-900">{invoice.invoiceNumber}</span>
                      </div>
                    </section>

                    <Divider />

                    <section className="grid gap-4">
                      <h3 className="text-xs font-semibold uppercase tracking-wider text-neutral-500">
                        {intl.formatMessage({
                          id: 'invoice.lineItems',
                          defaultMessage: 'Line items',
                        })}
                      </h3>
                      <div className="grid gap-6">
                        {invoice.items.map((item) => (
                          <div key={item.name} className="flex justify-between gap-4">
                            <div className="grid gap-0.5">
                              <p className="text-sm font-medium text-neutral-900">
                                {item.name}
                              </p>
                              <p className="text-xs text-neutral-500">
                                {intl.formatMessage(
                                  {
                                    id: 'invoice.quantity',
                                    defaultMessage: 'Qty {qty} × {price}',
                                  },
                                  {
                                    qty: item.quantity,
                                    price: formatMoney(
                                      intl,
                                      invoice.currency,
                                      item.unitPrice
                                    ),
                                  }
                                )}
                              </p>
                            </div>
                            <span className="text-sm font-semibold text-neutral-900">
                              {formatMoney(intl, invoice.currency, item.total)}
                            </span>
                          </div>
                        ))}
                      </div>
                    </section>

                    <Divider />

                    <section className="grid gap-2 text-sm text-neutral-600">
                      <div className="flex justify-between">
                        <span>{intl.formatMessage({ id: 'invoice.subtotal', defaultMessage: 'Subtotal' })}</span>
                        <span className="font-medium text-neutral-900">{formatMoney(intl, invoice.currency, invoice.subtotal)}</span>
                      </div>
                      <div className="flex justify-between">
                        <span>{intl.formatMessage({ id: 'invoice.tax', defaultMessage: 'Tax' })}</span>
                        <span className="font-medium text-neutral-900">{formatMoney(intl, invoice.currency, invoice.tax)}</span>
                      </div>
                      <div className="mt-2 flex justify-between text-base font-bold text-neutral-900">
                        <span>{intl.formatMessage({ id: 'invoice.total', defaultMessage: 'Total' })}</span>
                        <span>{formatMoney(intl, invoice.currency, invoice.total)}</span>
                      </div>
                    </section>
                  </div>
                </div>

                {/* Sticky Footer */}
                <div className="flex-none border-t border-neutral-100 p-6 bg-white/80 backdrop-blur">
                  <p className="text-sm text-neutral-500">
                    {intl.formatMessage({ id: 'invoice.questions', defaultMessage: 'Questions?' })}{' '}
                    <a href={`mailto:support@${invoice.orgName.toLowerCase().replace(/\s+/g, '')}.com`} className="font-medium text-blue-600 hover:text-blue-700">
                      {intl.formatMessage({ id: 'invoice.contactSupport', defaultMessage: 'Contact {orgName}' }, { orgName: invoice.orgName })}
                    </a>
                  </p>
                </div>
              </DrawerContent>
            </Drawer>
          )}
        </section>

        <footer className="text-center">
          <p className="flex items-center justify-center gap-1 text-sm font-medium text-neutral-400">
            {intl.formatMessage({ id: 'footer.poweredBy', defaultMessage: 'Powered by' })}
            <span className="font-bold text-neutral-600">Railzway</span>
          </p>
          <div className="mt-2 flex items-center justify-center gap-4 text-xs text-neutral-400">
            <a href="#" className="hover:text-neutral-600 hover:underline">
              {intl.formatMessage({ id: 'footer.terms', defaultMessage: 'Terms' })}
            </a>
            <a href="#" className="hover:text-neutral-600 hover:underline">
              {intl.formatMessage({ id: 'footer.privacy', defaultMessage: 'Privacy' })}
            </a>
          </div>
        </footer>
      </motion.div>

      <AnimatePresence>
        {stickyVisible ? (
          <motion.div
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 16 }}
            transition={{ duration: 0.25 }}
            className="fixed inset-x-4 bottom-4 z-20 rounded-2xl border border-neutral-200 bg-white/95 p-4 shadow-[0_16px_32px_-20px_rgba(15,23,42,0.35)] backdrop-blur"
          >
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-xs font-semibold uppercase tracking-[0.18em] text-neutral-500">
                  {intl.formatMessage({
                    id: 'invoice.totalDue',
                    defaultMessage: 'Total due',
                  })}
                </p>
                <p className="text-lg font-semibold text-neutral-900">
                  {formatMoney(intl, invoice.currency, invoice.total)}
                </p>
              </div>
              <motion.button
                className="rounded-xl bg-blue-600 px-4 py-3 text-sm font-semibold text-white"
                type="button"
                onClick={onProceed}
                whileHover={{ scale: 1.02 }}
                whileTap={{ scale: 0.98 }}
              >
                {intl.formatMessage({
                  id: 'invoice.proceedToPayment',
                  defaultMessage: 'Proceed to payment',
                })}
              </motion.button>
            </div>
          </motion.div>
        ) : null}
      </AnimatePresence>
    </div >
  )
}

function PaymentSummaryScreen({
  invoice,
  onChooseMethod,
}: {
  invoice: Invoice
  onChooseMethod: () => void
}) {
  const intl = useIntl()
  return (
    <CardShell
      title={invoice.orgName}
      subtitle={intl.formatMessage({
        id: 'payment.summary',
        defaultMessage: 'Payment summary',
      })}
    >
      <section className="text-center">
        <div className="text-3xl font-semibold text-neutral-900">
          {formatMoney(intl, invoice.currency, invoice.amountDue)}
        </div>
        <p className="mt-2 text-sm text-neutral-600">
          {intl.formatMessage(
            { id: 'payment.invoice', defaultMessage: 'Invoice {number}' },
            { number: invoice.invoiceNumber }
          )}
        </p>
        <p className="mt-4 text-sm text-neutral-500">
          {intl.formatMessage({
            id: 'payment.aboutToPay',
            defaultMessage: "You're about to pay this invoice.",
          })}
        </p>
      </section>

      <button
        className="w-full rounded-xl bg-blue-600 py-3 text-sm font-semibold text-white"
        type="button"
        onClick={onChooseMethod}
      >
        {intl.formatMessage({
          id: 'payment.chooseMethod',
          defaultMessage: 'Choose payment method',
        })}
      </button>
    </CardShell>
  )
}

function SelectMethodScreen({
  invoice,
  paymentMethods,
  selectedMethod,
  onSelect,
  onContinue,
  isBusy,
  errorMessage,
}: {
  invoice: Invoice
  paymentMethods: PublicPaymentMethod[]
  selectedMethod: PublicPaymentMethod | null
  onSelect: (method: PublicPaymentMethod) => void
  onContinue: () => void
  isBusy: boolean
  errorMessage: string | null
}) {
  const intl = useIntl()
  const hasMethods = paymentMethods.length > 0
  return (
    <CardShell
      title={invoice.orgName}
      subtitle={intl.formatMessage({
        id: 'payment.selectMethod',
        defaultMessage: 'Select payment method',
      })}
    >
      {hasMethods ? (
        <div className="grid gap-3">
          {paymentMethods.map((method) => {
            const isSelected =
              selectedMethod?.provider === method.provider &&
              selectedMethod?.type === method.type
            return (
              <button
                key={`${method.provider}-${method.type}`}
                type="button"
                onClick={() => onSelect(method)}
                disabled={isBusy}
                className={`flex items-center gap-3 rounded-xl border px-4 py-3 text-left text-sm transition-colors ${isSelected
                  ? 'border-blue-500 bg-blue-50'
                  : 'border-neutral-200 bg-white'
                  }`}
              >
                <span className="flex h-10 w-10 items-center justify-center rounded-lg bg-neutral-100 text-sm font-semibold text-neutral-700">
                  {method.display_name.slice(0, 1)}
                </span>
                <span className="flex-1">
                  <span className="block font-semibold text-neutral-900">
                    {method.display_name}
                  </span>
                  <span className="block text-xs text-neutral-500">
                    {method.type === 'card'
                      ? intl.formatMessage({
                        id: 'payment.cardPayment',
                        defaultMessage: 'Card payment',
                      })
                      : method.type === 'bank_transfer'
                        ? intl.formatMessage({
                          id: 'payment.bankTransfer',
                          defaultMessage: 'Bank transfer',
                        })
                        : intl.formatMessage({
                          id: 'payment.localPayment',
                          defaultMessage: 'Local payment',
                        })}
                  </span>
                </span>
                <span className="h-2 w-2 rotate-45 border-r-2 border-t-2 border-neutral-300" />
              </button>
            )
          })}
        </div>
      ) : (
        <div className="rounded-xl border border-neutral-200 bg-neutral-50 px-4 py-3 text-sm text-neutral-500">
          {intl.formatMessage({
            id: 'payment.noMethods',
            defaultMessage:
              'No payment methods are currently available for this invoice.',
          })}
        </div>
      )}

      {errorMessage ? (
        <p className="text-xs text-amber-700">{errorMessage}</p>
      ) : null}

      <button
        className="w-full rounded-xl bg-blue-600 py-3 text-sm font-semibold text-white disabled:bg-blue-300"
        type="button"
        onClick={onContinue}
        disabled={isBusy || !hasMethods}
      >
        {isBusy
          ? intl.formatMessage({
            id: 'payment.preparing',
            defaultMessage: 'Preparing...',
          })
          : intl.formatMessage({
            id: 'payment.continue',
            defaultMessage: 'Continue',
          })}
      </button>
    </CardShell>
  )
}

function CardFormScreen({
  invoice,
  clientSecret,
  isBusy,
  stripePromise,
  onBack,
  onProcessing,
}: {
  invoice: Invoice
  clientSecret: string | null
  isBusy: boolean
  stripePromise: StripePromise | null
  onBack: () => void
  onProcessing: () => void
}) {
  const intl = useIntl()
  const appearance = useMemo(
    () => ({
      theme: 'stripe',
      variables: {
        colorPrimary: '#1d5f9a',
        colorBackground: '#ffffff',
        colorText: '#0f172a',
        colorDanger: '#b45309',
        fontFamily:
          'Inter, SF Pro Text, SF Pro Display, Segoe UI, system-ui, sans-serif',
        borderRadius: '8px',
      },
    }),
    []
  )

  if (!stripePromise) {
    return (
      <CardShell
        title={invoice.orgName}
        subtitle={intl.formatMessage({
          id: 'payment.cardDetails',
          defaultMessage: 'Card payment',
        })}
      >
        <p className="text-sm text-neutral-600">
          {intl.formatMessage({
            id: 'payment.stripeMissing',
            defaultMessage:
              'Stripe is not configured. Add a publishable key for this organization or set VITE_STRIPE_PUBLISHABLE_KEY to enable card payments.',
          })}
        </p>
        <button
          className="w-full rounded-xl border border-neutral-200 bg-white py-3 text-sm font-semibold text-neutral-700"
          type="button"
          onClick={onBack}
        >
          {intl.formatMessage({
            id: 'payment.back',
            defaultMessage: 'Back',
          })}
        </button>
      </CardShell>
    )
  }

  return (
    <CardShell
      title={invoice.orgName}
      subtitle={intl.formatMessage({
        id: 'payment.cardDetails',
        defaultMessage: 'Card payment',
      })}
    >
      <div>
        <h2 className="text-base font-semibold text-neutral-900">
          {intl.formatMessage({
            id: 'payment.enterSecurely',
            defaultMessage: 'Enter your card details securely',
          })}
        </h2>
        <p className="mt-1 text-sm text-neutral-500">
          {intl.formatMessage({
            id: 'payment.neverStores',
            defaultMessage: 'We never store card information.',
          })}
        </p>
      </div>

      {!clientSecret ? (
        <div className="rounded-xl border border-neutral-200 bg-neutral-50 px-4 py-6 text-sm text-neutral-500">
          {intl.formatMessage({
            id: 'payment.preparingForm',
            defaultMessage: 'Preparing secure card form...',
          })}
        </div>
      ) : (
        <Elements
          stripe={stripePromise}
          options={{
            clientSecret,
            appearance: appearance as any,
          }}
        >
          <CardFormInner isBusy={isBusy} onBack={onBack} onProcessing={onProcessing} />
        </Elements>
      )}
    </CardShell>
  )
}

function CardFormInner({
  isBusy,
  onBack,
  onProcessing,
}: {
  isBusy: boolean
  onBack: () => void
  onProcessing: () => void
}) {
  const intl = useIntl()
  const stripe = useStripe()
  const elements = useElements()
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)

  const handleSubmit = async () => {
    if (!stripe || !elements) return
    setSubmitError(null)
    setIsSubmitting(true)
    const { error } = await stripe.confirmPayment({
      elements,
      confirmParams: {
        return_url: window.location.href,
      },
      redirect: 'if_required',
    })

    if (error) {
      setSubmitError(
        error.message ??
        intl.formatMessage({
          id: 'payment.confirmError',
          defaultMessage: 'Payment could not be confirmed.',
        })
      )
      setIsSubmitting(false)
      return
    }

    onProcessing()
  }

  return (
    <div className="grid gap-4">
      <div className="rounded-xl border border-neutral-200 bg-white px-3 py-4">
        <PaymentElement />
      </div>

      {submitError ? (
        <p className="text-xs text-amber-700">{submitError}</p>
      ) : null}

      <div className="grid gap-2">
        <button
          type="button"
          className="w-full rounded-xl bg-blue-600 py-3 text-sm font-semibold text-white disabled:bg-blue-300"
          onClick={handleSubmit}
          disabled={isSubmitting || isBusy || !stripe || !elements}
        >
          {isSubmitting
            ? intl.formatMessage({
              id: 'payment.processingShort',
              defaultMessage: 'Processing...',
            })
            : intl.formatMessage({
              id: 'payment.payNow',
              defaultMessage: 'Pay now',
            })}
        </button>
        <button
          type="button"
          className="w-full rounded-xl border border-neutral-200 bg-white py-3 text-sm font-semibold text-neutral-700"
          onClick={onBack}
          disabled={isSubmitting || isBusy}
        >
          {intl.formatMessage({
            id: 'payment.back',
            defaultMessage: 'Back',
          })}
        </button>
      </div>
    </div>
  )
}

function ProcessingScreen({
  invoice,
  paymentMethod,
}: {
  invoice: Invoice
  paymentMethod: PublicPaymentMethod | null
}) {
  const intl = useIntl()
  return (
    <CardShell
      title={invoice.orgName}
      subtitle={intl.formatMessage({
        id: 'payment.processing',
        defaultMessage: 'Processing payment...',
      })}
    >
      <div className="text-center">
        <div className="text-3xl font-semibold text-neutral-900">
          {formatMoney(intl, invoice.currency, invoice.amountDue)}
        </div>
        <p className="mt-2 text-sm text-neutral-500">
          {paymentMethodLabel(paymentMethod, intl)}
        </p>
      </div>
      <div className="grid gap-3">
        <div className="h-2 w-full rounded-full bg-neutral-100">
          <div className="h-2 w-2/3 rounded-full bg-blue-600" />
        </div>
        <div className="grid gap-1 text-center">
          <p className="text-sm text-neutral-600">
            {intl.formatMessage({
              id: 'payment.processing',
              defaultMessage: 'Processing payment...',
            })}
          </p>
          <p className="text-xs text-neutral-500">
            {intl.formatMessage({
              id: 'payment.pleaseWait',
              defaultMessage: 'Please wait while we process your payment.',
            })}
          </p>
        </div>
      </div>
    </CardShell>
  )
}

function SuccessScreen({
  invoice,
  paymentMethod,
}: {
  invoice: Invoice
  paymentMethod: PublicPaymentMethod | null
}) {
  const intl = useIntl()
  const methodSummary =
    paymentMethod?.display_name ?? paymentMethodLabel(paymentMethod, intl)
  const formattedTotal = formatMoney(intl, invoice.currency, invoice.amountDue)
  return (
    <CardShell
      title={invoice.orgName}
      subtitle={intl.formatMessage({
        id: 'success.receipt',
        defaultMessage: 'Receipt',
      })}
      footer={intl.formatMessage({
        id: 'success.poweredBy',
        defaultMessage: 'Powered by Valora',
      })}
    >
      <div className="flex items-center gap-3 rounded-xl border border-emerald-100 bg-emerald-50 px-4 py-3">
        <span className="flex h-10 w-10 items-center justify-center rounded-full bg-white text-emerald-600">
          <svg viewBox="0 0 24 24" className="h-5 w-5" aria-hidden="true">
            <path
              d="M5 12l4 4L19 6"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </span>
        <div>
          <p className="text-sm font-semibold text-neutral-900">
            {intl.formatMessage({
              id: 'success.title',
              defaultMessage: 'Payment Successful!',
            })}
          </p>
          <p className="text-sm text-neutral-600">
            {intl.formatMessage(
              {
                id: 'success.message',
                defaultMessage:
                  'Your payment of {amount} has been processed successfully.',
              },
              { amount: formattedTotal }
            )}
          </p>
        </div>
      </div>

      <div className="grid gap-2 text-sm text-neutral-600">
        <p>
          {intl.formatMessage(
            { id: 'success.invoice', defaultMessage: 'Invoice: {number}' },
            { number: invoice.invoiceNumber }
          )}
        </p>
        <p>
          {intl.formatMessage(
            { id: 'success.date', defaultMessage: 'Payment Date: {date}' },
            { date: invoice.paidDate ?? '—' }
          )}
        </p>
        <p>
          {intl.formatMessage(
            {
              id: 'payment.methodSummary',
              defaultMessage: 'Payment method: {method}',
            },
            { method: methodSummary }
          )}
        </p>
      </div>

      <div className="grid gap-2">
        <button className="w-full rounded-xl bg-blue-600 py-3 text-sm font-semibold text-white">
          {intl.formatMessage({
            id: 'success.downloadReceipt',
            defaultMessage: 'Download Receipt',
          })}
        </button>
        <button className="w-full rounded-xl border border-neutral-200 bg-white py-3 text-sm font-semibold text-neutral-700">
          {intl.formatMessage({
            id: 'success.anotherPayment',
            defaultMessage: 'Make Another Payment',
          })}
        </button>
      </div>
    </CardShell>
  )
}

function FailedScreen({
  onTryAgain,
  onChooseAnother,
}: {
  onTryAgain: () => void
  onChooseAnother: () => void
}) {
  const intl = useIntl()
  return (
    <CardShell
      title="Valora"
      subtitle={intl.formatMessage({
        id: 'payment.failedTitle',
        defaultMessage: 'Payment failed',
      })}
    >
      <div className="flex items-start gap-3 rounded-xl border border-amber-100 bg-amber-50 px-4 py-3">
        <span className="flex h-10 w-10 items-center justify-center rounded-full bg-white text-amber-700">
          <svg viewBox="0 0 24 24" className="h-5 w-5" aria-hidden="true">
            <path
              d="M12 7v6"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
            />
            <circle cx="12" cy="17" r="1" fill="currentColor" />
          </svg>
        </span>
        <div>
          <p className="text-sm font-semibold text-neutral-900">
            {intl.formatMessage({
              id: 'payment.failedHeadline',
              defaultMessage: 'Payment failed',
            })}
          </p>
          <p className="text-sm text-neutral-600">
            {intl.formatMessage({
              id: 'payment.failedMessage',
              defaultMessage:
                'The payment could not be completed. No charge was made.',
            })}
          </p>
        </div>
      </div>

      <div className="grid gap-2">
        <button
          className="w-full rounded-xl bg-blue-600 py-3 text-sm font-semibold text-white"
          type="button"
          onClick={onTryAgain}
        >
          {intl.formatMessage({
            id: 'payment.tryAgain',
            defaultMessage: 'Try again',
          })}
        </button>
        <button
          className="w-full rounded-xl border border-neutral-200 bg-white py-3 text-sm font-semibold text-neutral-700"
          type="button"
          onClick={onChooseAnother}
        >
          {intl.formatMessage({
            id: 'payment.chooseAnother',
            defaultMessage: 'Choose another method',
          })}
        </button>
      </div>
    </CardShell>
  )
}

function CardShell({
  title,
  subtitle,
  status,
  footer,
  children,
}: {
  title: string
  subtitle: string
  status?: string
  footer?: string
  children: ReactNode
}) {
  return (
    <div className="w-full rounded-2xl border border-neutral-200 bg-white p-6 shadow-[0_16px_32px_-28px_rgba(15,23,42,0.2)]">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-lg font-semibold text-neutral-900">{title}</p>
          <p className="mt-1 text-xs font-semibold uppercase tracking-[0.18em] text-neutral-500">
            {subtitle}
          </p>
        </div>
        {status ? (
          <span className="rounded-full bg-amber-50 px-3 py-1 text-[10px] font-semibold uppercase tracking-[0.18em] text-amber-700 ring-1 ring-amber-100">
            {status}
          </span>
        ) : null}
      </div>

      <div className="mt-6 grid gap-6">{children}</div>

      {footer ? (
        <div className="mt-6 border-t border-neutral-200 pt-4 text-xs text-neutral-500">
          {footer}
        </div>
      ) : null}
    </div>
  )
}

function Divider() {
  return <div className="h-px w-full bg-neutral-200" />
}

function KeyValue({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between">
      <span>{label}</span>
      <span className="font-semibold text-neutral-900">{value}</span>
    </div>
  )
}

function paymentMethodLabel(method: PublicPaymentMethod | null, intl: IntlShape) {
  if (!method) {
    return intl.formatMessage({
      id: 'payment.method',
      defaultMessage: 'Payment method',
    })
  }
  if (method.display_name) return method.display_name
  if (method.type === 'bank_transfer') {
    return intl.formatMessage({
      id: 'payment.bankTransfer',
      defaultMessage: 'Bank transfer',
    })
  }
  if (method.type === 'local_payment') {
    return intl.formatMessage({
      id: 'payment.localPayment',
      defaultMessage: 'Local payment',
    })
  }
  return intl.formatMessage({
    id: 'payment.cardPayment',
    defaultMessage: 'Card payment',
  })
}

async function fetchInvoiceStatus(
  orgId: string,
  token: string,
  signal?: AbortSignal
): Promise<InvoiceStatus | null> {
  if (!token || !orgId) return null
  const response = await fetch(
    `/public/orgs/${orgId}/invoices/${token}/status`,
    { signal }
  )
  if (!response.ok) return null
  const data = (await response.json()) as Partial<{ status: InvoiceStatus }>
  return data.status ?? null
}

function getPublicInvoiceRoute() {
  if (typeof window === 'undefined') {
    return { orgId: '', token: '' }
  }
  const params = new URLSearchParams(window.location.search)
  const queryToken = params.get('token') ?? ''
  const queryOrg = params.get('org_id') ?? ''
  const segments = window.location.pathname.split('/').filter(Boolean)
  const token = queryToken || segments[segments.length - 1] || ''
  const orgId = queryOrg || segments[segments.length - 2] || ''
  return { orgId, token }
}

function mapInvoicePayload(
  payload: PublicInvoiceResponse['invoice'],
  intl: IntlShape
): Invoice {
  return {
    orgName: payload.org_name,
    invoiceNumber: payload.invoice_number,
    issueDate: formatDate(intl, payload.issue_date),
    dueDate: formatDate(intl, payload.due_date),
    dueDateRaw: payload.due_date,
    paidDate: payload.paid_date ? formatDate(intl, payload.paid_date) : undefined,
    paidDateRaw: payload.paid_date ?? undefined,
    paymentState: payload.payment_state ?? undefined,
    invoiceStatus: payload.invoice_status ?? undefined,
    billToName: payload.bill_to_name,
    billToEmail: payload.bill_to_email,
    currency: payload.currency,
    amountDue: payload.amount_due,
    subtotal: payload.subtotal_amount,
    tax: payload.tax_amount,
    total: payload.total_amount,
    items: payload.items.map((item) => ({
      name: item.description,
      quantity: item.quantity,
      unitPrice: item.unit_price,
      total: item.amount,
      lineType: item.line_type,
    })),
  }
}

function formatMoney(intl: IntlShape, currency: string, amount: number) {
  const value = amount / 100
  return intl.formatNumber(value, {
    style: 'currency',
    currency,
  })
}

function formatDate(intl: IntlShape, value: string) {
  if (!value) return value
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return intl.formatDate(date, {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  })
}

function derivePaymentBadge(invoice: Invoice, intl: IntlShape) {
  const status = (invoice.invoiceStatus ?? '').toUpperCase()
  const paidAt = parseDate(invoice.paidDateRaw)
  const dueAt = parseDate(invoice.dueDateRaw)
  const now = new Date()

  if (status === 'VOID') {
    return {
      label: intl.formatMessage({
        id: 'invoice.status.void',
        defaultMessage: 'Voided',
      }),
      state: 'void',
      tone: 'danger',
    }
  }
  if (paidAt) {
    return {
      label: intl.formatMessage({
        id: 'invoice.status.paid',
        defaultMessage: 'Paid',
      }),
      state: 'paid',
      tone: 'success',
    }
  }
  if (status === 'DRAFT') {
    return {
      label: intl.formatMessage({
        id: 'invoice.status.draft',
        defaultMessage: 'Draft',
      }),
      state: 'draft',
      tone: 'muted',
    }
  }
  if (status === 'FINALIZED' && dueAt && dueAt < now) {
    return {
      label: intl.formatMessage({
        id: 'invoice.status.overdue',
        defaultMessage: 'Overdue',
      }),
      state: 'overdue',
      tone: 'warning',
    }
  }
  return {
    label: intl.formatMessage({
      id: 'invoice.status.open',
      defaultMessage: 'Open',
    }),
    state: 'open',
    tone: 'muted',
  }
}

function parseDate(value?: string) {
  if (!value) return null
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return null
  return date
}

async function readPublicInvoiceError(
  response: Response
): Promise<PublicInvoiceErrorResponse | null> {
  try {
    const data = (await response.json()) as PublicInvoiceErrorResponse
    if (data && (data.code || data.message)) {
      return data
    }
  } catch {
    // ignore invalid json
  }
  return null
}

function PublicInvoiceErrorScreen({
  title,
  message,
  onClose,
}: {
  title: string
  message: string
  onClose: () => void
}) {
  const intl = useIntl()
  return (
    <motion.section
      initial={{ opacity: 0, scale: 0.98 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={{ duration: 0.25 }}
      className="rounded-2xl border border-neutral-200 bg-white p-6 text-neutral-900 shadow-[0_16px_32px_-28px_rgba(15,23,42,0.2)] dark:border-neutral-800 dark:bg-neutral-950 dark:text-neutral-100"
    >
      <h1 className="text-xl font-semibold">{title}</h1>
      <p className="mt-3 text-sm text-neutral-600 dark:text-neutral-300">
        {message}
      </p>
      <p className="mt-2 text-sm text-neutral-500 dark:text-neutral-400">
        {intl.formatMessage({
          id: 'invoice.error.contactSender',
          defaultMessage: 'Please contact the sender to request a new invoice link.',
        })}
      </p>
      <div className="mt-6">
        <button
          type="button"
          className="w-full rounded-xl bg-blue-600 py-3 text-sm font-semibold text-white"
          onClick={onClose}
        >
          {intl.formatMessage({
            id: 'payment.back',
            defaultMessage: 'Back',
          })}
        </button>
      </div>
    </motion.section>
  )
}
