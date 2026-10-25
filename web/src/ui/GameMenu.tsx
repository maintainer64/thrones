import { useEffect, useRef, useState } from "react";
import {
  BookOpen,
  GraduationCap,
  Languages,
  Map,
  Settings,
  Settings2,
  Sparkles,
  Volume2,
  VolumeX,
  Wifi,
  WifiOff,
} from "lucide-react";
import { t } from "../i18n/core";

export function GameMenu({
  language,
  sound,
  motion,
  online,
  inRoom,
  onLanguage,
  onSound,
  onMotion,
  onTutorial,
  onRoomSettings,
  onRules,
  onBoard,
}: {
  language: string;
  sound: boolean;
  motion: boolean;
  online: boolean;
  inRoom: boolean;
  onLanguage: () => void;
  onSound: () => void;
  onMotion: () => void;
  onTutorial: () => void;
  onRoomSettings: () => void;
  onRules: () => void;
  onBoard: () => void;
}) {
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (!open) return;
    root.current
      ?.querySelector<HTMLButtonElement>(".game-menu-panel button")
      ?.focus();
    const outside = (event: PointerEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", outside);
    return () => document.removeEventListener("pointerdown", outside);
  }, [open]);
  const action = (callback: () => void) => {
    setOpen(false);
    trigger.current?.focus();
    callback();
  };
  return (
    <div
      className="game-menu"
      ref={root}
      onKeyDown={(event) => {
        if (event.key === "Escape" && open) {
          event.stopPropagation();
          setOpen(false);
          trigger.current?.focus();
        }
      }}
      onBlur={(event) => {
        if (
          event.relatedTarget &&
          !event.currentTarget.contains(event.relatedTarget)
        )
          setOpen(false);
      }}
    >
      <button
        ref={trigger}
        className="game-menu-toggle"
        data-testid="game-menu-toggle"
        data-tour="help"
        aria-label={t("settings.openMenu")}
        title={t("settings.openMenu")}
        aria-expanded={open}
        aria-haspopup="dialog"
        aria-controls="game-menu-panel"
        onClick={() => setOpen(!open)}
      >
        <Settings size={21} />
      </button>
      {open && (
        <section
          id="game-menu-panel"
          className="game-menu-panel"
          role="dialog"
          aria-labelledby="game-menu-title"
        >
          <h2 id="game-menu-title">{t("settings.menu")}</h2>
          <button
            data-testid="language-switch"
            aria-label={t("settings.changeLanguage")}
            onClick={() => action(onLanguage)}
          >
            <Languages size={19} />
            <span>{t("settings.language")}</span>
            <b>
              {language.toUpperCase()} → {language === "ru" ? "EN" : "RU"}
            </b>
          </button>
          <button
            aria-label={t("settings.sound")}
            aria-pressed={sound}
            onClick={() => action(onSound)}
          >
            {sound ? <Volume2 size={19} /> : <VolumeX size={19} />}
            <span>{t("settings.sound")}</span>
            <small>{sound ? t("settings.on") : t("settings.off")}</small>
          </button>
          <button
            aria-label={t("settings.animations")}
            aria-pressed={motion}
            onClick={() => action(onMotion)}
          >
            <Sparkles size={19} />
            <span>{t("settings.animations")}</span>
            <small>{motion ? t("settings.on") : t("settings.off")}</small>
          </button>
          {inRoom && (
            <button
              aria-label={t("room.settings")}
              onClick={() => action(onRoomSettings)}
            >
              <Settings2 size={19} />
              <span>{t("room.settings")}</span>
            </button>
          )}
          <div className="game-menu-divider" />
          <button onClick={() => action(onBoard)}>
            <Map size={19} />
            <span>{t("navigation.board")}</span>
          </button>
          <button onClick={() => action(onRules)}>
            <BookOpen size={19} />
            <span>{t("navigation.rules")}</span>
          </button>
          {inRoom && (
            <button
              aria-label={t("tutorial.open")}
              onClick={() => action(onTutorial)}
            >
              <GraduationCap size={19} />
              <span>{t("tutorial.open")}</span>
            </button>
          )}
          {inRoom && (
            <p className={`game-menu-connection ${online ? "live" : ""}`}>
              {online ? <Wifi size={15} /> : <WifiOff size={15} />}
              {online ? t("connection.connected") : t("connection.waiting")}
            </p>
          )}
        </section>
      )}
    </div>
  );
}
