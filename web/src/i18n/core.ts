import ru from "../locales/ru.json";
import en from "../locales/en.json";

export type Language = "ru" | "en";
export type TranslationKey = keyof typeof ru;
export const storageKey = "westeros-language";
const listeners = new Set<() => void>();
function initialLanguage(): Language {
  if (typeof window === "undefined") return "ru";
  try {
    const value =
      window.localStorage.getItem(storageKey) ||
      new URLSearchParams(window.location.search).get("lang");
    return value === "en" ? "en" : "ru";
  } catch {
    return "ru";
  }
}
let language: Language = initialLanguage();
export const getLanguage = () => language;
export function defaultRoomTitle(value: Language) {
  return (value === "en" ? en : ru)["room.defaultTitle"];
}
export function setLanguage(value: Language) {
  if (value !== "ru" && value !== "en") return;
  language = value;
  if (typeof document !== "undefined") document.documentElement.lang = value;
  try {
    if (typeof window !== "undefined")
      window.localStorage.setItem(storageKey, value);
  } catch {
    /* Private browsing may disable storage. */
  }
  listeners.forEach((listener) => listener());
}
export function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
export function t(key: TranslationKey, params: Record<string, unknown> = {}) {
  const dictionary: Record<TranslationKey, string> =
    language === "en" ? en : ru;
  return dictionary[key].replace(/\{(p\d+)\}/g, (match, key: string) =>
    key in params ? String(params[key]) : match,
  );
}
if (typeof document !== "undefined") document.documentElement.lang = language;
if (typeof window !== "undefined")
  window.addEventListener("storage", (event) => {
    if (
      event.key === storageKey &&
      (event.newValue === "ru" || event.newValue === "en") &&
      event.newValue !== language
    )
      setLanguage(event.newValue);
  });
