type ApiError = {
  message?: string
  response?: {
    status?: number
    data?: {
      error?: {
        message?: string
      }
    }
  }
}

export const getErrorMessage = (err: unknown, fallback: string) => {
  const apiError = err as ApiError
  const apiMessage = apiError?.response?.data?.error?.message
  if (typeof apiMessage === "string" && apiMessage.trim()) {
    return apiMessage
  }
  const message = apiError?.message
  if (typeof message === "string" && message.trim()) {
    return message
  }
  return fallback
}

export const isForbiddenError = (err: unknown) => {
  const apiError = err as ApiError
  return apiError?.response?.status === 403
}
