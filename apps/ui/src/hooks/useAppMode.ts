type AppMode = "oss" | "cloud"

const readEnvMode = (): AppMode | null => {
  const raw = import.meta.env.VITE_APP_MODE
  if (raw === "oss" || raw === "cloud") {
    return raw
  }
  return null
}

export function useAppMode(): AppMode {
  return readEnvMode() ?? "oss"
}
