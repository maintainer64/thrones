import { Crown, Eye, ScrollText, Swords } from "lucide-react";
import { t } from "../i18n/core";

export function phaseStep(phase: string) {
  if (phase === "lobby") return -1;
  if (phase === "planning") return 0;
  if (phase === "orders_revealed" || phase === "raven") return 1;
  if (
    [
      "raids",
      "marches",
      "combat",
      "retreat",
      "card_effect",
      "consolidate_power",
    ].includes(phase)
  )
    return 2;
  return 3;
}
export function PhaseSteps({ phase }: { phase: string }) {
  const current = phaseStep(phase);
  const steps = [
    { name: t("phases.planning"), icon: ScrollText },
    { name: t("phases.revealed"), icon: Eye },
    { name: t("ui.actionPhase"), icon: Swords },
    { name: t("phases.westeros"), icon: Crown },
  ];
  return (
    <ol className="phase-steps" aria-label={t("ui.phaseSequence")}>
      {steps.map(({ name, icon: Icon }, index) => (
        <li
          key={index}
          aria-current={index === current ? "step" : undefined}
          data-complete={current > index}
          title={name}
        >
          <Icon size={14} aria-hidden="true" />
          <span>{name}</span>
        </li>
      ))}
    </ol>
  );
}
