import { t } from "./i18n/core";
import "@fontsource-variable/alegreya/wght.css";
import "@fontsource-variable/alegreya/wght-italic.css";
import { createRoot } from "react-dom/client";
import { lazy, Suspense } from "react";
const App = lazy(() => import("./App"));
createRoot(document.getElementById("root")!).render(
  <Suspense
    fallback={<div className="scene-boot">{t("loading.westeros")}</div>}
  >
    <App />
  </Suspense>,
);
