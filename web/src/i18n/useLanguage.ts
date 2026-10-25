import { useSyncExternalStore } from "react";
import { getLanguage, subscribe } from "./core";
export function useLanguage() {
  return useSyncExternalStore(subscribe, getLanguage, () => "ru" as const);
}
