import { useState } from 'react'
import DropIn from 'braintree-web-drop-in-react'

type BraintreeCheckoutProps = {
  authorization: string
  onSuccess: (nonce: string) => void
  onFailure: (error: any) => void
}

export function BraintreeCheckout({
  authorization,
  onSuccess,
  onFailure,
}: BraintreeCheckoutProps) {
  const [instance, setInstance] = useState<any>(null)
  const [isProcessing, setIsProcessing] = useState(false)

  const handlePayment = async () => {
    if (!instance) return
    setIsProcessing(true)
    try {
      const { nonce } = await instance.requestPaymentMethod()
      onSuccess(nonce)
    } catch (error) {
      console.error(error)
      onFailure(error)
    } finally {
      setIsProcessing(false)
    }
  }

  if (!authorization) return null

  return (
    <div className="w-full">
      <DropIn
        options={{ authorization }}
        onInstance={setInstance}
      />
      <button
        type="button"
        className="mt-4 w-full rounded-xl bg-blue-600 py-3 text-sm font-semibold text-white disabled:bg-neutral-300"
        onClick={handlePayment}
        disabled={!instance || isProcessing}
      >
        {isProcessing ? 'Processing...' : 'Pay Now'}
      </button>
    </div>
  )
}
