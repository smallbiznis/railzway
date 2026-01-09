import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { IntlProvider } from "react-intl";
import "./styles/index.css";
import App from "./App.tsx";
import { DEFAULT_LOCALE, resolveLocale, translations } from "./i18n";

const locale = resolveLocale();
const messages = translations[locale] ?? translations[DEFAULT_LOCALE];

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <IntlProvider locale={locale} defaultLocale={DEFAULT_LOCALE} messages={messages}>
      <App />
    </IntlProvider>
  </StrictMode>,
)

  
