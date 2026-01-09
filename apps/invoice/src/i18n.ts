export const translations = {
  en: {
    "invoice.title": "Invoice",
    "invoice.status.open": "OPEN",
    "invoice.status.paid": "PAID",
    "invoice.totalDue": "TOTAL DUE",
    "invoice.dueOn": "Due on {date}",
    "invoice.number": "Invoice number",
    "invoice.issueDate": "Issue date",
    "invoice.dueDate": "Due date",
    "invoice.proceedToPayment": "Proceed to payment",
    "invoice.downloadPDF": "Download PDF",
    "invoice.copyLink": "Copy invoice link",
    "invoice.billTo": "BILL TO",
    "invoice.lineItems": "LINE ITEMS",
    "invoice.quantity": "Qty {qty} at {price}",
    "invoice.subtotal": "Subtotal",
    "invoice.tax": "Tax",
    "invoice.total": "Total",
    "payment.title": "Payment",
    "payment.summary": "PAYMENT SUMMARY",
    "payment.invoice": "Invoice {number}",
    "payment.aboutToPay": "You're about to pay this invoice.",
    "payment.chooseMethod": "Choose payment method",
    "payment.selectMethod": "SELECT PAYMENT METHOD",
    "payment.stripe": "Stripe",
    "payment.cardPayment": "Card payment",
    "payment.continue": "Continue",
    "payment.back": "Back",
    "payment.cardDetails": "CARD PAYMENT",
    "payment.enterSecurely": "Enter your card details securely",
    "payment.neverStores": "We never store card information.",
    "payment.cardNumber": "Card number",
    "payment.expiryDate": "Expiry date",
    "payment.cvc": "CVC",
    "payment.country": "Country",
    "payment.selectCountry": "Select country",
    "payment.payNow": "Pay now",
    "payment.processing": "Processing payment...",
    "payment.pleaseWait": "Please wait while we process your payment.",
    "success.title": "Payment Successful!",
    "success.message": "Your payment of {amount} has been processed successfully.",
    "success.invoice": "Invoice: {number}",
    "success.date": "Payment Date: {date}",
    "success.downloadReceipt": "Download Receipt",
    "success.backToDashboard": "Back to Dashboard",
    "success.anotherPayment": "Make Another Payment"
  },
  id: {
    "invoice.title": "Faktur",
    "invoice.status.open": "TERBUKA",
    "invoice.status.paid": "LUNAS",
    "invoice.totalDue": "TOTAL TAGIHAN",
    "invoice.dueOn": "Jatuh tempo {date}",
    "invoice.number": "Nomor faktur",
    "invoice.issueDate": "Tanggal terbit",
    "invoice.dueDate": "Tanggal jatuh tempo",
    "invoice.proceedToPayment": "Lanjutkan ke pembayaran",
    "invoice.downloadPDF": "Unduh PDF",
    "invoice.copyLink": "Salin tautan faktur",
    "invoice.billTo": "TAGIHAN UNTUK",
    "invoice.lineItems": "ITEM",
    "invoice.quantity": "Jml {qty} @ {price}",
    "invoice.subtotal": "Subtotal",
    "invoice.tax": "Pajak",
    "invoice.total": "Total",
    "payment.title": "Pembayaran",
    "payment.summary": "RINGKASAN PEMBAYARAN",
    "payment.invoice": "Faktur {number}",
    "payment.aboutToPay": "Anda akan membayar faktur ini.",
    "payment.chooseMethod": "Pilih metode pembayaran",
    "payment.selectMethod": "PILIH METODE PEMBAYARAN",
    "payment.stripe": "Stripe",
    "payment.cardPayment": "Pembayaran kartu",
    "payment.continue": "Lanjutkan",
    "payment.back": "Kembali",
    "payment.cardDetails": "PEMBAYARAN KARTU",
    "payment.enterSecurely": "Masukkan detail kartu Anda dengan aman",
    "payment.neverStores": "Kami tidak pernah menyimpan informasi kartu.",
    "payment.cardNumber": "Nomor kartu",
    "payment.expiryDate": "Tanggal kedaluwarsa",
    "payment.cvc": "CVC",
    "payment.country": "Negara",
    "payment.selectCountry": "Pilih negara",
    "payment.payNow": "Bayar sekarang",
    "payment.processing": "Memproses pembayaran...",
    "payment.pleaseWait": "Mohon tunggu sementara kami memproses pembayaran Anda.",
    "success.title": "Pembayaran Berhasil!",
    "success.message": "Pembayaran Anda sebesar {amount} telah berhasil diproses.",
    "success.invoice": "Faktur: {number}",
    "success.date": "Tanggal Pembayaran: {date}",
    "success.downloadReceipt": "Unduh Tanda Terima",
    "success.backToDashboard": "Kembali ke Dasbor",
    "success.anotherPayment": "Lakukan Pembayaran Lain"
  }
} as const;

export type Locale = keyof typeof translations;

export const DEFAULT_LOCALE: Locale = "en";

export function resolveLocale(): Locale {
  if (typeof window === "undefined") {
    return DEFAULT_LOCALE;
  }

  const params = new URLSearchParams(window.location.search);
  const requested = (params.get("lang") ?? params.get("locale") ?? "").toLowerCase();
  if (requested.startsWith("id")) return "id";
  if (requested.startsWith("en")) return "en";

  const navigatorLocale =
    (navigator.languages && navigator.languages.length > 0
      ? navigator.languages[0]
      : navigator.language) ?? "";

  if (navigatorLocale.toLowerCase().startsWith("id")) return "id";
  return DEFAULT_LOCALE;
}
