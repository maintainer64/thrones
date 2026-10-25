import { useContext, useEffect, useRef } from "react";
import type { ReactNode } from "react";
import { DecisionFlowContext } from "./decision-flow";
import { t } from "../i18n/core";

const focusable =
  'button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), a[href], summary, [tabindex="0"]';

export function Modal({
  children,
  titleID,
  className = "",
  focusKey,
  focusReady = true,
  onClose,
}: {
  children: ReactNode;
  titleID: string;
  className?: string;
  focusKey?: string;
  focusReady?: boolean;
  onClose: () => void;
}) {
  const ref = useRef<HTMLElement>(null);
  const flow = useContext(DecisionFlowContext);
  useEffect(() => {
    const previous = document.activeElement;
    return () => {
      if (previous instanceof HTMLElement && previous.isConnected)
        previous.focus();
    };
  }, []);
  useEffect(() => {
    // A WS snapshot may advance the step while its HTTP command is still in
    // flight. Wait for the new controls to become enabled before focusing.
    if (!focusReady) return;
    const root = ref.current;
    const target = root?.querySelector<HTMLElement>(
      "[data-initial-focus]:not(:disabled)",
    );
    (target || root)?.focus();
  }, [focusKey, focusReady]);
  return (
    <div className="modal-backdrop influence-backdrop">
      <section
        ref={ref}
        className={`battle-modal influence-modal ${className}`}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleID}
        tabIndex={-1}
        onKeyDown={(event) => {
          if (event.key === "Escape") {
            event.stopPropagation();
            onClose();
          }
          if (event.key !== "Tab") return;
          const targets = Array.from(
            event.currentTarget.querySelectorAll<HTMLElement>(focusable),
          ).filter((node) => node.getClientRects().length > 0);
          const first = targets[0];
          const last = targets.at(-1);
          if (!first || !last) {
            event.preventDefault();
            ref.current?.focus();
          } else if (
            !targets.includes(document.activeElement as HTMLElement) ||
            document.activeElement === (event.shiftKey ? first : last)
          ) {
            event.preventDefault();
            (event.shiftKey ? last : first).focus();
          }
        }}
      >
        {children}
        {(flow?.stale || flow?.urgent) && (
          <footer className="decision-next" role="status">
            <p>{t("dialogs.updatedWhileReading")}</p>
            <button type="button" className="primary" onClick={flow.next}>
              {t("dialogs.next")}
            </button>
            {flow.urgent && (
              <button onClick={flow.urgent}>{t("combat.yourDecision")}</button>
            )}
          </footer>
        )}
      </section>
    </div>
  );
}
