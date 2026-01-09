/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_APP_MODE?: "oss" | "cloud"
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
