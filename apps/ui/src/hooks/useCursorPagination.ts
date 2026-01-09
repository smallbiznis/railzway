import { type DependencyList, useCallback, useEffect, useMemo, useRef, useState } from "react"

type RawPageInfo = {
  has_next?: boolean
  has_prev?: boolean
  next_cursor?: string | null
  prev_cursor?: string | null
  has_more?: boolean
  next_page_token?: string | null
  previous_page_token?: string | null
}

type NormalizedPageInfo = {
  has_next: boolean
  has_prev: boolean
  next_cursor: string | null
  prev_cursor: string | null
}

export type CursorPage<T> = {
  items: T[]
  page_info?: RawPageInfo | null
}

export type CursorFetchDirection = "initial" | "next" | "prev"

export type CursorFetcher<T> = (
  cursor: string | null,
  direction: CursorFetchDirection
) => Promise<CursorPage<T>>

type CursorPaginationOptions = {
  enabled?: boolean
  dependencies?: DependencyList
  mode?: "append" | "replace"
}

const emptyPageInfo: NormalizedPageInfo = {
  has_next: false,
  has_prev: false,
  next_cursor: null,
  prev_cursor: null,
}

const normalizePageInfo = (pageInfo?: RawPageInfo | null): NormalizedPageInfo => {
  if (!pageInfo) return emptyPageInfo
  const nextCursor = pageInfo.next_cursor ?? pageInfo.next_page_token ?? null
  const prevCursor = pageInfo.prev_cursor ?? pageInfo.previous_page_token ?? null
  const hasNext = pageInfo.has_next ?? pageInfo.has_more ?? Boolean(nextCursor)
  const hasPrev = pageInfo.has_prev ?? Boolean(prevCursor)

  return {
    has_next: Boolean(hasNext),
    has_prev: Boolean(hasPrev),
    next_cursor: nextCursor,
    prev_cursor: prevCursor,
  }
}

export function useCursorPagination<T>(
  fetcher: CursorFetcher<T>,
  options: CursorPaginationOptions = {}
) {
  const { enabled = true, dependencies = [], mode = "append" } = options
  const [items, setItems] = useState<T[]>([])
  const [pageInfo, setPageInfo] = useState<NormalizedPageInfo>(emptyPageInfo)
  const [isLoading, setIsLoading] = useState(false)
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const [cursorStack, setCursorStack] = useState<(string | null)[]>([null])
  const [cursorIndex, setCursorIndex] = useState(0)
  const requestIdRef = useRef(0)
  const mountedRef = useRef(true)

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
    }
  }, [])

  const applyPage = useCallback(
    (page: CursorPage<T>, mode: "replace" | "append" | "prepend") => {
      const normalizedItems = Array.isArray(page.items) ? page.items : []
      setPageInfo(normalizePageInfo(page.page_info))

      if (mode === "replace") {
        setItems(normalizedItems)
        return
      }
      if (mode === "append") {
        setItems((prev) => [...prev, ...normalizedItems])
        return
      }
      setItems((prev) => [...normalizedItems, ...prev])
    },
    []
  )

  const loadInitial = useCallback(async () => {
    if (!enabled) return
    const requestId = ++requestIdRef.current
    setIsLoading(true)
    setError(null)
    setItems([])
    setPageInfo(emptyPageInfo)
    if (mode === "replace") {
      setCursorStack([null])
      setCursorIndex(0)
    }

    try {
      const page = await fetcher(null, "initial")
      if (!mountedRef.current || requestIdRef.current !== requestId) return
      applyPage(page, "replace")
    } catch (err) {
      if (!mountedRef.current || requestIdRef.current !== requestId) return
      setError(err)
      setItems([])
      setPageInfo(emptyPageInfo)
    } finally {
      if (!mountedRef.current || requestIdRef.current !== requestId) return
      setIsLoading(false)
    }
  }, [applyPage, enabled, fetcher])

  const loadNext = useCallback(async () => {
    if (!enabled || isLoading || isLoadingMore) return
    const storedCursor =
      mode === "replace" ? cursorStack[cursorIndex + 1] ?? null : null
    const nextCursor = storedCursor ?? pageInfo.next_cursor
    if (mode === "append") {
      if (!pageInfo.has_next || !pageInfo.next_cursor) return
    } else if (!storedCursor && (!pageInfo.has_next || !pageInfo.next_cursor)) {
      return
    }
    if (!nextCursor) return
    const requestId = ++requestIdRef.current
    setIsLoadingMore(true)
    setError(null)

    try {
      const page = await fetcher(nextCursor, "next")
      if (!mountedRef.current || requestIdRef.current !== requestId) return
      applyPage(page, mode === "append" ? "append" : "replace")
      if (mode === "replace") {
        if (!storedCursor) {
          setCursorStack((prev) => {
            const next = prev.slice(0, cursorIndex + 1)
            next.push(nextCursor)
            return next
          })
        }
        setCursorIndex((prev) => prev + 1)
      }
    } catch (err) {
      if (!mountedRef.current || requestIdRef.current !== requestId) return
      setError(err)
    } finally {
      if (!mountedRef.current || requestIdRef.current !== requestId) return
      setIsLoadingMore(false)
    }
  }, [
    applyPage,
    cursorIndex,
    cursorStack,
    enabled,
    fetcher,
    isLoading,
    isLoadingMore,
    mode,
    pageInfo,
  ])

  const loadPrev = useCallback(async () => {
    if (!enabled || isLoading || isLoadingMore) return
    const prevCursor =
      mode === "replace" ? cursorStack[cursorIndex - 1] : pageInfo.prev_cursor
    if (mode === "append") {
      if (!pageInfo.has_prev || !pageInfo.prev_cursor) return
    } else if (cursorIndex <= 0) {
      return
    }
    if (!prevCursor) return
    const requestId = ++requestIdRef.current
    setIsLoadingMore(true)
    setError(null)

    try {
      const page = await fetcher(prevCursor, "prev")
      if (!mountedRef.current || requestIdRef.current !== requestId) return
      applyPage(page, mode === "append" ? "prepend" : "replace")
      if (mode === "replace") {
        setCursorIndex((prev) => Math.max(prev - 1, 0))
      }
    } catch (err) {
      if (!mountedRef.current || requestIdRef.current !== requestId) return
      setError(err)
    } finally {
      if (!mountedRef.current || requestIdRef.current !== requestId) return
      setIsLoadingMore(false)
    }
  }, [
    applyPage,
    cursorIndex,
    cursorStack,
    enabled,
    fetcher,
    isLoading,
    isLoadingMore,
    mode,
    pageInfo,
  ])

  const reset = useCallback(() => {
    requestIdRef.current += 1
    setItems([])
    setPageInfo(emptyPageInfo)
    setError(null)
    setIsLoading(false)
    setIsLoadingMore(false)
    setCursorStack([null])
    setCursorIndex(0)
  }, [])

  useEffect(() => {
    if (!enabled) {
      reset()
      return
    }
    void loadInitial()
  }, [enabled, loadInitial, reset, ...dependencies])

  const hasNext = useMemo(
    () => {
      if (mode === "replace") {
        if (cursorIndex < cursorStack.length - 1) return true
        return Boolean(pageInfo.has_next && pageInfo.next_cursor)
      }
      return Boolean(pageInfo.has_next && pageInfo.next_cursor)
    },
    [cursorIndex, cursorStack.length, mode, pageInfo]
  )
  const hasPrev = useMemo(
    () => {
      if (mode === "replace") {
        return cursorIndex > 0
      }
      return Boolean(pageInfo.has_prev && pageInfo.prev_cursor)
    },
    [cursorIndex, mode, pageInfo]
  )

  return {
    items,
    error,
    isLoading,
    isLoadingMore,
    hasNext,
    hasPrev,
    nextCursor: pageInfo.next_cursor,
    prevCursor: pageInfo.prev_cursor,
    loadNext,
    loadPrev,
    reset,
    reload: loadInitial,
  }
}
