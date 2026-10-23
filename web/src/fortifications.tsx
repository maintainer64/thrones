import { Shield, LockKeyhole } from "lucide-react";
import { t } from "./i18n/core";
import { FortificationKind } from "./enums";
import type { Fortification } from "./types";

export function fortificationName(value: Fortification) {
  if (value.kind === FortificationKind.Blocked)
    return t("fortifications.blocked");
  if (value.kind === FortificationKind.Garrison)
    return t("fortifications.garrison");
  return t("fortifications.neutral");
}
export function FortificationBadge({ value }: { value: Fortification }) {
  return (
    <span
      className="fortification-badge"
      data-fortification={value.kind}
      title={`${fortificationName(value)}${value.kind === FortificationKind.Blocked ? "" : `: ${value.strength}`}`}
    >
      {value.kind === FortificationKind.Blocked ? (
        <LockKeyhole size={14} />
      ) : (
        <Shield size={18} />
      )}
      <b>{value.kind === FortificationKind.Blocked ? "∞" : value.strength}</b>
      {value.house && <img src={`/media/crests/${value.house}.svg`} alt="" />}
    </span>
  );
}
export function FortificationInfo({ value }: { value: Fortification }) {
  return (
    <section
      className="fortification-info"
      aria-label={fortificationName(value)}
    >
      <details>
        <summary>
          <FortificationBadge value={value} />
          <b>{fortificationName(value)}</b>
        </summary>
        <p>
          {value.kind === FortificationKind.Blocked
            ? t("fortifications.blockedHint")
            : value.kind === FortificationKind.Garrison
              ? t("fortifications.garrisonHint", { p0: value.strength })
              : t("fortifications.neutralHint", { p0: value.strength })}
        </p>
      </details>
    </section>
  );
}
