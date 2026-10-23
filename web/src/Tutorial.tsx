import { t } from "./i18n/core";
import { useEffect, useLayoutEffect, useState } from "react";
import { ArrowLeft, ArrowRight, GraduationCap, X } from "lucide-react";
function getSteps() {
  return [
    {
      target: "houses",
      title: t("tutorial.housesTitle"),
      text: t("tutorial.housesBody"),
    },
    {
      target: "board",
      title: t("tutorial.boardTitle"),
      text: t("tutorial.boardBody"),
    },
    {
      target: "inspector",
      title: t("tutorial.ordersTitle"),
      text: t("tutorial.ordersBody"),
    },
    {
      target: "phase",
      title: t("tutorial.phasesTitle"),
      text: t("tutorial.phasesBody"),
    },
    {
      target: "timer",
      title: t("settings.decisionTime"),
      text: t("tutorial.timerBody"),
    },
    {
      target: "actions",
      title: t("tutorial.actionsTitle"),
      text: t("tutorial.actionsBody"),
    },
    {
      target: "cards",
      title: t("tutorial.cardsTitle"),
      text: t("tutorial.cardsBody"),
    },
    {
      target: "help",
      title: t("room.returnAnyTime"),
      text: t("tutorial.helpBody"),
    },
  ];
}
export function Tutorial({ onClose }: { onClose: () => void }) {
  const steps = getSteps();
  const [index, setIndex] = useState(0);
  const [rect, setRect] = useState<DOMRect>();
  const step = steps[index];
  useLayoutEffect(() => {
    const update = () => {
      const el = document.querySelector(`[data-tour="${step.target}"]`);
      setRect(el?.getBoundingClientRect());
    };
    update();
    window.addEventListener("resize", update);
    document.addEventListener("scroll", update, true);
    return () => {
      window.removeEventListener("resize", update);
      document.removeEventListener("scroll", update, true);
    };
  }, [index]);
  useEffect(() => {
    const listener = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", listener);
    return () => document.removeEventListener("keydown", listener);
  }, [onClose]);
  return (
    <div
      className="tour-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="tour-title"
    >
      <div className="tour-shade" />
      {rect && rect.width > 0 && (
        <div
          className="tour-highlight"
          style={{
            left: Math.max(4, rect.left - 5),
            top: Math.max(4, rect.top - 5),
            width: Math.min(rect.width + 10, window.innerWidth - 8),
            height: Math.min(rect.height + 10, window.innerHeight - 8),
          }}
        />
      )}
      <article className="tour-card">
        <div className="tour-top">
          <GraduationCap />
          <span>
            {t("tutorial.stepPrefix")}
            {index + 1} / {steps.length}
          </span>
          <button aria-label={t("tutorial.close")} onClick={onClose}>
            <X size={18} />
          </button>
        </div>
        <h2 id="tour-title">{step.title}</h2>
        <p>{step.text}</p>
        <div className="tour-progress">
          {steps.map((_, i) => (
            <span key={i} className={i <= index ? "done" : ""} />
          ))}
        </div>
        <footer>
          <button disabled={index === 0} onClick={() => setIndex((i) => i - 1)}>
            <ArrowLeft size={16} />
            {t("common.back")}
          </button>
          <button
            className="primary"
            onClick={() =>
              index === steps.length - 1 ? onClose() : setIndex((i) => i + 1)
            }
          >
            {index === steps.length - 1
              ? t("navigation.play")
              : t("common.next")}
            <ArrowRight size={16} />
          </button>
        </footer>
      </article>
    </div>
  );
}
