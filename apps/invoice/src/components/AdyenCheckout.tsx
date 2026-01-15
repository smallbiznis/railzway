import { useEffect, useRef } from 'react'
import { AdyenCheckout as AdyenWeb } from '@adyen/adyen-web'
import '@adyen/adyen-web/styles/adyen.css'

type AdyenCheckoutProps = {
  sessionId: string
  sessionData: string
  environment: string
  clientKey: string
  onSuccess: () => void
  onFailure: (error: any) => void
}

export function AdyenCheckout({
  sessionId,
  sessionData,
  environment,
  clientKey,
  onSuccess,
  onFailure,
}: AdyenCheckoutProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const checkoutRef = useRef<any>(null)

  useEffect(() => {
    if (!containerRef.current || !sessionId || !sessionData) return

    const initAdyen = async () => {
      try {
        const configuration = {
          environment, // 'test' || 'live'
          clientKey,
          session: {
            id: sessionId,
            sessionData: sessionData,
          },
          onPaymentCompleted: (result: any) => {
            if (['Authorised', 'Received', 'Pending'].includes(result.resultCode)) {
              onSuccess()
            } else {
              onFailure(new Error(result.resultCode || 'Payment failed'))
            }
          },
          onError: (error: any) => {
            console.error(error)
            onFailure(error)
          },
        }

        const checkout = await AdyenWeb(configuration as any)
        checkoutRef.current = checkout

          // Create 'dropin' component
          ; (checkout as any).create('dropin', {
            // Drop-in specific config if needed
            showPayButton: true,
          }).mount(containerRef.current)

      } catch (error) {
        console.error('Adyen init error', error)
        onFailure(error)
      }
    }

    initAdyen()

    return () => {
      // Cleanup if needed, though Adyen doesn't strictly require unmount call usually
      // if (checkoutRef.current) ...
    }
  }, [sessionId, sessionData, environment, clientKey])

  return <div ref={containerRef} className="adyen-checkout-container w-full" />
}
