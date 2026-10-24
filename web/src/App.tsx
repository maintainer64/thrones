import { defaultRoomTitle, setLanguage, t } from "./i18n/core";
import { useLanguage } from "./i18n/useLanguage";
import { CardChoiceKind, TidesDecision } from "./enums";
import { lazy, Suspense, useEffect, useMemo, useRef, useState } from "react";
import {
  ArrowRight,
  Check,
  ChevronRight,
  Clock,
  Copy,
  Crown,
  Feather,
  Flag,
  HelpCircle,
  Pause,
  Play,
  Shield,
  Swords,
  X,
} from "lucide-react";
import { Tutorial } from "./Tutorial";
import { OrderDescription, OrderPalette } from "./OrderPalette";
import { MapActionPanel } from "./MapActionPanel";
import { FortificationInfo } from "./fortifications";
import { DecisionFlowContext, useDecisionFlow } from "./ui/decision-flow";
import { HouseCard } from "./cards/HouseCard";
import { HouseHand } from "./cards/HouseHand";
import { HousePicker } from "./lobby/HousePicker";
import { CardInspector } from "./cards/CardInspector";
import { CombatReport } from "./cards/CombatReport";
import { InfluencePanel } from "./influence/InfluencePanel";
import { PhaseSteps } from "./ui/PhaseSteps";
import { GameMenu } from "./ui/GameMenu";
import { HouseSeat } from "./ui/HouseSeat";
import { OrderToken } from "./ui/OrderToken";
import { OrderBudget } from "./ui/OrderBudget";
import { Modal } from "./ui/Modal";
import { CombatIntro } from "./combat/CombatIntro";
import { PlaybackControls } from "./combat/PlaybackControls";
import { useAnimation } from "./combat/useAnimation";
import { StrengthBreakdown } from "./combat/StrengthBreakdown";
import {
  hasCombatDecision,
  useCombatPresentation,
} from "./combat/presentation";
import { TidesCard } from "./cards/TidesCard";
import { InfluenceDialog, InfluenceReport } from "./influence/InfluenceDialog";
import { ProvisionDialog } from "./provision/ProvisionDialog";
import { EventDialog } from "./events/EventDialog";
import { RavenDialog } from "./raven/RavenDialog";
import { ravenIdentity } from "./raven/model";
import { eventsIdentity, eventText as et } from "./events/model";
import { textureKey } from "./cards/card-art";
import { originalPortrait } from "./artwork/artwork";
import {
  api,
  castleCount,
  defaultSettings,
  getPhases,
  regionName,
  getUnitNames,
} from "./types";
import type { Assets, Catalog, Room, Settings } from "./types";
import { keepNewest } from "./state";
import type { GameManifest } from "./scene3d/game-state";
import type { MapAction } from "./scene3d/game-state";
import { isGameManifest } from "./scene3d/game-state";
import "./style.css";
import "./game.css";
import "./scene3d/game-board.css";
import "./cards/cards.css";
import "./hud.css";
import "./strategy-theme.css";
import "./panels.css";
const GameBoard3D = lazy(() => import("./scene3d/GameBoard3D"));
function getExpansions() {
  return [
    {
      id: "base",
      title: t("expansions.baseTitle"),
      description: t("expansions.baseDescription"),
      available: true,
    },
    {
      id: "dance",
      title: t("expansions.danceTitle"),
      description: t("expansions.danceDescription"),
      available: false,
    },
    {
      id: "feast",
      title: t("expansions.feastTitle"),
      description: t("expansions.feastDescription"),
      available: false,
    },
    {
      id: "mother",
      title: t("expansions.motherTitle"),
      description: t("expansions.motherDescription"),
      available: false,
    },
  ];
}
function Crest({ house, size = 32 }: { house: string; size?: number }) {
  return (
    <img
      className="crest"
      src={`/media/crests/${house}.svg`}
      width={size}
      height={size}
      alt=""
    />
  );
}
function RoomSettings({
  value,
  onChange,
  disabled = false,
}: {
  value: Settings;
  onChange: (value: Settings) => void;
  disabled?: boolean;
}) {
  return (
    <div className="room-settings">
      <div className="setting-row">
        <label htmlFor="player-count">
          {t("setup.playerCount")}
          <small>{t("setup.playerCountHint")}</small>
        </label>
        <select
          id="player-count"
          aria-label={t("setup.playerCount")}
          value={value.playerCount || 6}
          disabled={disabled}
          onChange={(event) =>
            onChange({ ...value, playerCount: Number(event.target.value) })
          }
        >
          {[3, 4, 5, 6].map((count) => (
            <option key={count} value={count}>
              {t("setup.housesCount", { p0: count })}
            </option>
          ))}
        </select>
      </div>
      <div className="setting-row">
        <label htmlFor="turn-minutes">
          {t("settings.decisionTime")}
          <small>{t("settings.decisionTimeHint")}</small>
        </label>
        <div className="minute-input">
          <input
            id="turn-minutes"
            type="number"
            min={1}
            max={120}
            value={value.turnMinutes}
            disabled={disabled}
            onChange={(e) =>
              onChange({ ...value, turnMinutes: Number(e.target.value) })
            }
          />
          <span>{t("timer.minutesUnit")}</span>
        </div>
      </div>
      <label className="toggle-row">
        <input
          type="checkbox"
          checked={value.hardTimer}
          disabled={disabled}
          onChange={(e) => onChange({ ...value, hardTimer: e.target.checked })}
        />
        <span>
          {t("settings.hardTimer")}
          <small>
            {value.hardTimer
              ? t("settings.hardTimerHint")
              : t("settings.softTimerHint")}
          </small>
        </span>
      </label>
      <label className="toggle-row">
        <input
          type="checkbox"
          checked={value.tidesOfBattle || false}
          disabled={disabled}
          onChange={(event) =>
            onChange({ ...value, tidesOfBattle: event.target.checked })
          }
        />
        <span>
          {t("tides.setting")}
          <small>{t("tides.settingHint")}</small>
        </span>
      </label>
    </div>
  );
}
function GameClock({ room }: { room: Room }) {
  const [now, setNow] = useState(Date.now());
  const offset = useMemo(
    () => Date.parse(room.serverTime || "") - Date.now(),
    [room.serverTime],
  );
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, []);
  const deadline = Date.parse(room.clock?.deadline || "");
  const valid = Number.isFinite(deadline) && deadline > 0;
  const diff = valid
    ? Math.ceil(
        (deadline - now - (Number.isFinite(offset) ? offset : 0)) / 1000,
      )
    : 0;
  const remaining = Math.abs(diff);
  return (
    <div
      className={`turn-clock ${valid && diff <= 0 ? "overtime" : ""}`}
      data-tour="timer"
      title={
        room.settings.hardTimer
          ? t("timer.hardLimitHint")
          : t("timer.softLimitHint")
      }
    >
      <Clock size={17} />
      <div>
        <b>
          {valid
            ? `${diff < 0 ? "+" : ""}${String(Math.floor(remaining / 60)).padStart(2, "0")}:${String(remaining % 60).padStart(2, "0")}`
            : "—:—"}
        </b>
        <small>
          {valid && diff < 0
            ? t("timer.overtime")
            : room.settings.hardTimer
              ? t("timer.hardLimit")
              : t("timer.softLimit")}
        </small>
      </div>
    </div>
  );
}
export default function App() {
  const language = useLanguage();
  const expansions = getExpansions();
  const phases = getPhases();
  const unitNames = getUnitNames();
  const [gameManifest, setGameManifest] = useState<GameManifest>();
  const [gameAssetsError, setGameAssetsError] = useState("");
  const [gameAssetsLoading, setGameAssetsLoading] = useState(true);
  const [catalog, setCatalog] = useState<Catalog>();
  const [assets, setAssets] = useState<Assets>();
  const [room, setRoom] = useState<Room>();
  const [roomID, setRoomID] = useState(
    new URLSearchParams(location.search).get("room") || "",
  );
  const [name, setName] = useState(localStorage.getItem("westeros-name") || "");
  const [title, setTitle] = useState(t("room.defaultTitle"));
  const [chosenHouse, setChosenHouse] = useState("stark");
  const [invitation, setInvitation] = useState<{
    id: string;
    phase: string;
    seats: { house: string; available: boolean }[];
  }>();
  const [settings, setSettings] = useState<Settings>(defaultSettings);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const pending = useRef(false);
  const [online, setOnline] = useState(false);
  const [selected, setSelected] = useState("winterfell");
  const [target, setTarget] = useState("");
  const [retreatTarget, setRetreatTarget] = useState("");
  const ownRetreat =
    !!room &&
    room.phase === "retreat" &&
    (room.battle?.retreatChooser || room.battle?.loser) === room.yourHouse;
  const [moving, setMoving] = useState(false);
  const [frameRequest, setFrameRequest] = useState(0);
  const [quantities, setQuantities] = useState<Record<string, number>>({});
  const [leaveToken, setLeaveToken] = useState(false);
  const [preview, setPreview] = useState<{
    key: string;
    status: "loading" | "ready" | "error";
    targets: string[];
    error: string;
  }>({ key: "", status: "loading", targets: [], error: "" });
  const [previewRetry, setPreviewRetry] = useState(0);
  const selectedTroops =
    (room?.match ? room.match.armies : catalog?.units)?.[selected] || [];
  const actionAvailable =
    !!room &&
    room.match?.activeHouse === room.yourHouse &&
    ["marches", "raids"].includes(room.phase) &&
    room.legalRegions?.includes(selected);
  const previewKey = JSON.stringify([
    room?.id,
    room?.revision,
    selected,
    selectedTroops.map((unit) => [
      unit.unitType,
      quantities[unit.unitType] ?? unit.quantity,
    ]),
    leaveToken,
  ]);
  const previewReady = preview.key === previewKey && preview.status === "ready";
  const previewTargets = previewReady ? preview.targets : [];
  const [tab, setTab] = useState("board");
  const [tour, setTour] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [sound, setSound] = useState(
    localStorage.getItem("westeros-sound") === "true",
  );
  const [motion, setMotion] = useState(
    localStorage.getItem("westeros-motion") !== "false",
  );
  const [copied, setCopied] = useState(false);
  const [showResult, setShowResult] = useState(false);
  const [showInfluenceResult, setShowInfluenceResult] = useState(false);
  const ravenID = room ? ravenIdentity(room) : "";
  const ravenActive = !!ravenID;
  const eventIdentity = room ? eventsIdentity(room) : "";
  const eventsActive = !!eventIdentity;
  const provisionActive =
    !!room && ["supply", "mustering", "consolidate_power"].includes(room.phase);
  const [inspectedCard, setInspectedCard] = useState<string>();
  const effectDialog = useRef<HTMLElement>(null);
  useEffect(() => {
    if (
      room?.phase === "card_effect" &&
      room.battle?.choice?.house === room.yourHouse
    )
      effectDialog.current
        ?.querySelector<HTMLButtonElement>("button:not(:disabled)")
        ?.focus();
  }, [
    room?.phase,
    room?.battle?.choice?.house,
    room?.battle?.choice?.card,
    room?.battle?.choice?.kind,
  ]);
  const [housesOpen, setHousesOpen] = useState(false);
  const [inspectorOpen, setInspectorOpen] = useState(true);
  const [tracksOpen, setTracksOpen] = useState(false);
  useEffect(() => {
    if (inspectorOpen) setTracksOpen(false);
  }, [inspectorOpen]);
  const [handOpen, setHandOpen] = useState(true);
  const [toast, setToast] = useState("");
  const combatPresentation = useCombatPresentation(room);
  const animateCombat = useAnimation(motion);
  const flow = useDecisionFlow(
    room,
    settingsOpen ||
      !!inspectedCard ||
      tour ||
      showResult ||
      showInfluenceResult ||
      combatPresentation.pending,
  );
  const dialogRoom = flow.active?.room;
  const eventsOpen = flow.active?.kind === "events";
  const ravenOpen = flow.active?.kind === "raven";
  const provisionOpen = flow.active?.kind === "provision";
  const influenceOpen = flow.active?.kind === "influence";
  const setEventsOpen = (open: boolean) =>
    open ? flow.open("events") : eventsOpen && flow.close();
  const setRavenOpen = (open: boolean) =>
    open ? flow.open("raven") : ravenOpen && flow.close();
  const setProvisionOpen = (open: boolean) =>
    open ? flow.open("provision") : provisionOpen && flow.close();
  const setInfluenceOpen = (open: boolean) =>
    open ? flow.open("influence") : influenceOpen && flow.close();
  const lastEvent = useRef<number | undefined>(undefined);
  useEffect(() => {
    const close = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setInspectedCard(undefined);
        setSettingsOpen(false);
        setShowResult(false);
        setMoving(false);
        setTarget("");
        setRetreatTarget("");
      }
    };
    window.addEventListener("keydown", close);
    return () => window.removeEventListener("keydown", close);
  }, []);
  function accept(next: Room) {
    setRoom((old) => keepNewest(old, next));
  }
  function enter(next: Room) {
    localStorage.setItem("westeros-name", name.trim());
    setRoomID(next.id);
    accept(next);
    const url = new URL(location.href);
    url.searchParams.set("room", next.id);
    history.replaceState(null, "", url);
  }
  async function loadGameAssets() {
    setGameAssetsLoading(true);
    setGameAssetsError("");
    try {
      const response = await fetch(
        "/local-assets/preview3d/game-manifest.json",
        { cache: "no-cache" },
      );
      if (!response.ok) throw Error(t("assets.preparedModelsMissing"));
      const manifest = await response.json();
      if (!isGameManifest(manifest)) throw Error(t("assets.incompatible"));
      setGameManifest(manifest);
    } catch (error) {
      setGameAssetsError(
        error instanceof Error ? error.message : String(error),
      );
    } finally {
      setGameAssetsLoading(false);
    }
  }
  useEffect(() => {
    let live = true;
    document.title = t("app.title");
    setTitle((previous) =>
      previous === defaultRoomTitle("ru") || previous === defaultRoomTitle("en")
        ? defaultRoomTitle(language)
        : previous,
    );
    void api<Catalog>(`/api/catalog?lang=${language}`)
      .then((catalog) => {
        if (live) setCatalog(catalog);
      })
      .catch((error) => {
        if (live) setError(error.message);
      });
    return () => {
      live = false;
    };
  }, [language]);
  useEffect(() => {
    void fetch("/media/manifest.json")
      .then((r) => {
        if (!r.ok) throw Error(t("assets.boardMissing"));
        return r.json() as Promise<Assets>;
      })
      .then((a) => {
        setAssets(a);
      })
      .catch((e) => setError(e.message));
    void loadGameAssets();
  }, []);
  useEffect(() => {
    if (!/^[a-f0-9]{16}$/.test(roomID)) return;
    let live = true;
    void api<Room>(`/api/rooms/${encodeURIComponent(roomID)}`)
      .then((r) => {
        if (live) accept(r);
      })
      .catch((e) => {
        // A new invitee is expected not to have a seat yet.
        if (live && e.status !== 403) setError(e.message);
      });
    return () => {
      live = false;
    };
  }, [roomID, language]);
  useEffect(() => {
    if (room || !/^[a-f0-9]{16}$/.test(roomID)) return;
    let live = true;
    let timer: ReturnType<typeof setTimeout>;
    const refresh = async () => {
      try {
        const lobby = await api<NonNullable<typeof invitation>>(
          `/api/rooms/${roomID}/lobby`,
        );
        if (!live) return;
        setInvitation({ ...lobby, id: roomID });
        setChosenHouse((previous) =>
          lobby.seats.some((seat) => seat.house === previous && seat.available)
            ? previous
            : lobby.seats.find((seat) => seat.available)?.house || "",
        );
      } catch (error) {
        if (live)
          setError(error instanceof Error ? error.message : String(error));
      } finally {
        if (live) timer = setTimeout(refresh, 10000);
      }
    };
    const debounce = setTimeout(refresh, 200);
    return () => {
      live = false;
      clearTimeout(debounce);
      clearTimeout(timer);
    };
  }, [roomID, !!room]);
  const activeID = room?.id;
  useEffect(() => {
    if (!activeID) return;
    let stopped = false;
    let delay = 1000;
    let timer: ReturnType<typeof setTimeout>;
    let socket: WebSocket;
    let refreshPending = false;
    let lastRefresh = 0;
    const catchUp = () => {
      if (
        stopped ||
        document.visibilityState !== "visible" ||
        refreshPending ||
        Date.now() - lastRefresh < 1000
      )
        return;
      lastRefresh = Date.now();
      refreshPending = true;
      void api<Room>(`/api/rooms/${activeID}`)
        .then((next) => {
          if (!stopped) accept(next);
        })
        .catch(() => {})
        .finally(() => {
          refreshPending = false;
        });
    };
    const connect = () => {
      socket = new WebSocket(
        `${location.protocol === "https:" ? "wss" : "ws"}://${location.host}/api/rooms/${activeID}/ws?lang=${language}`,
      );
      socket.onopen = () => {
        if (stopped) return;
        setOnline(true);
        delay = 1000;
      };
      socket.onmessage = (e) => {
        if (stopped) return;
        try {
          const next = JSON.parse(e.data);
          if (next.id === activeID && typeof next.revision === "number")
            accept(next);
        } catch {
          setError(t("connection.updateError"));
        }
      };
      socket.onclose = () => {
        if (!stopped) {
          setOnline(false);
          timer = setTimeout(connect, delay + Math.random() * 500);
          delay = Math.min(delay * 2, 15000);
        }
      };
      socket.onerror = () => socket.close();
    };
    connect();
    window.addEventListener("focus", catchUp);
    document.addEventListener("visibilitychange", catchUp);
    return () => {
      stopped = true;
      window.removeEventListener("focus", catchUp);
      document.removeEventListener("visibilitychange", catchUp);
      clearTimeout(timer);
      socket.close();
    };
  }, [activeID, language]);
  useEffect(() => {
    const event = room?.events?.at(-1);
    if (!event) return;
    if (lastEvent.current !== undefined && lastEvent.current !== event.id) {
      setToast(event.text);
      if (sound) {
        const file =
          event.type === "combat"
            ? "combat"
            : event.type === "march"
              ? "horse-gallop"
              : "notification";
        const audio = new Audio(`/media/sounds/${file}.ogg`);
        audio.volume = 0.3;
        void audio.play().catch(() => {});
      }
    }
    lastEvent.current = event.id;
    const timer = setTimeout(() => setToast(""), 5000);
    return () => clearTimeout(timer);
  }, [room?.events?.at(-1)?.id, sound]);
  useEffect(() => {
    setRetreatTarget("");
    if (ownRetreat && room?.battle) {
      setSelected(room.battle.to);
      setShowResult(false);
      setInspectedCard(undefined);
      setTab("board");
    }
  }, [room?.id, ownRetreat, room?.battle?.to]);
  useEffect(() => {
    if (
      room &&
      room.phase !== "lobby" &&
      !room.influence &&
      !provisionActive &&
      !eventsActive &&
      !ravenActive &&
      !eventsOpen &&
      !localStorage.getItem("westeros-tour-v2")
    ) {
      setHousesOpen(true);
      setInspectorOpen(true);
      setHandOpen(true);
      setTour(true);
      localStorage.setItem("westeros-tour-v2", "true");
    }
  }, [room?.phase]);
  useEffect(() => {
    setMoving(
      !!room &&
        room.match?.activeHouse === room.yourHouse &&
        ["marches", "raids"].includes(room.phase) &&
        !!room.legalRegions?.includes(selected),
    );
    setTarget("");
    setQuantities({});
    setLeaveToken(false);
  }, [selected, room?.phase, room?.match?.activeHouse]);
  useEffect(() => {
    if (!moving || !actionAvailable || !room) return;
    const controller = new AbortController();
    setTarget("");
    setPreview({ key: previewKey, status: "loading", targets: [], error: "" });
    const units = Object.fromEntries(
      selectedTroops.map((unit) => [
        unit.unitType,
        quantities[unit.unitType] ?? unit.quantity,
      ]),
    );
    if (
      room.phase === "marches" &&
      Object.values(units).every((n) => n === 0)
    ) {
      setPreview({ key: previewKey, status: "ready", targets: [], error: "" });
      return;
    }
    const timer = setTimeout(() => {
      void fetch(`/api/rooms/${room.id}/preview`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Accept-Language": language,
        },
        signal: controller.signal,
        body: JSON.stringify({
          type: room.phase === "raids" ? "raid" : "march",
          region: selected,
          revision: room.revision,
          units,
          leaveToken,
        }),
      })
        .then(async (response) => {
          const body = await response.json();
          if (!response.ok) {
            if (response.status === 409 && !controller.signal.aborted)
              void api<Room>(`/api/rooms/${room.id}`)
                .then(accept)
                .catch(() => {});
            throw Error(body.error || t("actions.targetsError"));
          }
          if (!controller.signal.aborted && body.revision === room.revision)
            setPreview({
              key: previewKey,
              status: "ready",
              targets: body.targets,
              error: "",
            });
        })
        .catch((error) => {
          if (!controller.signal.aborted)
            setPreview({
              key: previewKey,
              status: "error",
              targets: [],
              error: error.message,
            });
        });
    }, 160);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [moving, actionAvailable, previewKey, previewRetry]);
  async function run(task: () => Promise<void>) {
    if (pending.current) return;
    pending.current = true;
    setBusy(true);
    setError("");
    try {
      await task();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      if (roomID)
        void api<Room>(`/api/rooms/${encodeURIComponent(roomID)}`)
          .then(accept)
          .catch(() => {});
    } finally {
      pending.current = false;
      setBusy(false);
    }
  }
  function send(
    command: Record<string, unknown>,
    onSuccess?: (next: Room) => void,
  ) {
    if (!room) return;
    void run(async () => {
      const next = await api<Room>(`/api/rooms/${room.id}/commands`, {
        ...command,
        revision: room.revision,
      });
      accept(next);
      setMoving(false);
      setTarget("");
      onSuccess?.(next);
    });
  }
  function sendDecision(
    command: Record<string, unknown>,
    onSuccess?: () => void,
  ) {
    if (flow.stale) return;
    send(command, (next) => {
      flow.commit(next);
      onSuccess?.();
    });
  }
  function select(id: string) {
    setSelected(id);
    setInspectorOpen(true);
    setMoving(
      !!room &&
        room.match?.activeHouse === room.yourHouse &&
        ["marches", "raids"].includes(room.phase) &&
        !!room.legalRegions?.includes(id),
    );
    setTarget("");
  }
  function confirmMapAction() {
    if (
      !actionAvailable ||
      !previewReady ||
      !previewTargets.includes(target) ||
      !room
    )
      return;
    send({
      type: room.phase === "raids" ? "raid" : "march",
      region: selected,
      target,
      units: Object.fromEntries(
        selectedTroops.map((unit) => [
          unit.unitType,
          quantities[unit.unitType] ?? unit.quantity,
        ]),
      ),
      leaveToken,
    });
  }
  function passMapAction() {
    if (actionAvailable) send({ type: "pass", region: selected });
  }
  function art(card: string, house: string) {
    const key = textureKey(card, house);
    return originalPortrait(key);
  }
  if (!catalog || !assets)
    return (
      <div className="loading-screen">
        <Crown size={44} />
        <h1>{t("room.creating")}</h1>
        <p>{error || t("loading.boardPiecesCards")}</p>
      </div>
    );
  const c = catalog;
  const a = assets;
  const armies = room?.match?.armies || room?.setup?.armies || c.units || {};
  const region = c.board.regions.find((r) => r.id === selected);
  const army = armies?.[selected] || [];
  const ours = army?.some((u) => u.house === room?.yourHouse);
  const me = room?.seats.find((s) => s.you);
  const myTurn = room?.match?.activeHouse === room?.yourHouse;
  const battle = room?.battle;
  const cardChoice = battle?.choice;
  const allCards = Object.values(c.houses).flatMap((h) =>
    Object.entries(h.houseCards),
  );
  const choiceCard = cardChoice
    ? allCards.find(([id]) => id === cardChoice.card)?.[1]
    : undefined;
  const isTidesChoice =
    cardChoice?.kind === CardChoiceKind.TidesRedraw ||
    cardChoice?.kind === CardChoiceKind.TidesBlade;
  const isBladeChoice = cardChoice?.kind === CardChoiceKind.Blade;
  const choiceLabel = (value: string) =>
    isTidesChoice || isBladeChoice
      ? value === TidesDecision.Redraw
        ? t("tides.redraw")
        : value === TidesDecision.UseBlade
          ? t("tides.useBlade")
          : t("tides.keep")
      : cardChoice?.kind === CardChoiceKind.Region
        ? regionName(c.board.regions.find((r) => r.id === value))
        : cardChoice?.kind === CardChoiceKind.Track
          ? {
              ironThrone: t("tracks.ironThrone"),
              fiefdoms: t("tracks.fiefdoms"),
              kingsCourt: t("tracks.kingsCourt"),
            }[value] || value
          : cardChoice?.kind === CardChoiceKind.Confirm
            ? value === "cancel"
              ? t("ability.cancelEnemyCard")
              : t("ability.chooseEnemyRetreat")
            : allCards.find(([id]) => id === value)?.[1].name || value;
  const inBattle =
    !!room &&
    !!battle &&
    [battle.attacker, battle.defender].includes(room.yourHouse);
  const order = c.orders.find((o) => o.id === room?.orders[selected]);
  const myRegions = Object.entries(armies)
    .filter(([, units]) => units?.some((u) => u.house === room?.yourHouse))
    .map(([id]) => id);
  const planned = myRegions.filter((id) => room?.orders[id]).length;
  const mapAction: MapAction | undefined =
    actionAvailable && room
      ? {
          kind: room.phase === "raids" ? "raid" : "march",
          selecting: moving,
          loading:
            moving &&
            (preview.key !== previewKey || preview.status === "loading"),
          error: preview.key === previewKey ? preview.error : "",
          targets: previewTargets,
          busy,
          units: selectedTroops.map((unit) => ({
            type: unit.unitType,
            label: unitNames[unit.unitType],
            available: unit.quantity,
            quantity: quantities[unit.unitType] ?? unit.quantity,
          })),
          onQuantity: (type, value) =>
            setQuantities((old) => ({ ...old, [type]: value })),
          onBegin: () => {
            setMoving(true);
            setTarget("");
          },
          onCancel: () => {
            setMoving(false);
            setTarget("");
          },
          onConfirm: confirmMapAction,
          onPass: passMapAction,
          onRetry: () => setPreviewRetry((value) => value + 1),
        }
      : undefined;
  return (
    <DecisionFlowContext.Provider
      value={
        flow.active
          ? {
              stale: flow.stale,
              next: flow.next,
              urgent: hasCombatDecision(room)
                ? () => {
                    flow.close();
                    combatPresentation.toDecision();
                  }
                : undefined,
            }
          : null
      }
    >
      <div
        className={`app immersive ${!room && tab === "board" ? "welcome-screen" : ""} ${room && tab === "board" ? "game-hud" : ""} ${motion ? "" : "no-motion"}`}
        data-room-id={room?.id}
        data-room-revision={room?.revision}
        data-room-phase={room?.phase}
        data-combat-playback={combatPresentation.onMap}
        data-houses-open={housesOpen}
        data-inspector-open={inspectorOpen}
        data-hand-open={handOpen}
        data-tracks-open={tracksOpen}
      >
        <GameMenu
          language={language}
          sound={sound}
          motion={motion}
          online={online}
          inRoom={!!room}
          onLanguage={() => setLanguage(language === "ru" ? "en" : "ru")}
          onSound={() => {
            localStorage.setItem("westeros-sound", String(!sound));
            setSound(!sound);
          }}
          onMotion={() => {
            localStorage.setItem("westeros-motion", String(!motion));
            setMotion(!motion);
          }}
          onRoomSettings={() => {
            if (!room) return;
            setSettings(room.settings || defaultSettings);
            setSettingsOpen(true);
          }}
          onTutorial={() => {
            setTab("board");
            setHousesOpen(true);
            setInspectorOpen(true);
            setHandOpen(true);
            setTour(true);
          }}
          onRules={() => setTab("rules")}
          onBoard={() => setTab("board")}
        />
        {error && (
          <div className="error" role="alert">
            {error}
            <button
              aria-label={t("common.dismissError")}
              onClick={() => setError("")}
            >
              <X size={16} />
            </button>
          </div>
        )}
        {tab === "rules" ? (
          <main className="rules">
            <p className="eyebrow">{t("rules.eyebrow")}</p>
            <h1>{t("rules.title")}</h1>
            <p>{t("rules.serverAuthority")}</p>
            <div className="rule-grid">
              {expansions.map((e) => (
                <article key={e.id}>
                  <ScrollIcon />
                  <h2>{e.title}</h2>
                  <p>{e.description}</p>
                  <strong>
                    {e.available
                      ? t("rules.prototypeAvailable")
                      : t("rules.notImplemented")}
                  </strong>
                </article>
              ))}
            </div>
            <h2>{t("rules.implementedTitle")}</h2>
            <p>{t("rules.implementedBody")}</p>
            <h2>{t("rules.missingTitle")}</h2>
            <p>{t("rules.missingBody")}</p>
            <h2>{t("rules.tutorialTitle")}</h2>
            <p>{t("rules.tutorialBody")}</p>
            <p>
              {t("rules.officialPdfsPrefix")}
              <code>local-rulebooks/</code>
              {t("rules.sourcesPrefix")}
              <code>docs/rules/catalog.json</code>
              {t("rules.artAttribution")}
            </p>
            <button className="primary" onClick={() => setTab("board")}>
              {t("navigation.returnToTable")}
              <ArrowRight size={16} />
            </button>
          </main>
        ) : !room ? (
          <main className="welcome enhanced-welcome">
            <div className="welcome-story">
              <p className="eyebrow">{t("welcome.slogan")}</p>
              <h1>
                {t("welcome.winterIsComing")}
                <br />
                <em>{t("turn.yourTurnNotice")}</em>
              </h1>
              <p>
                {t("welcome.inviteFriends")}
                <br />
                {t("welcome.secretDecisionsLead")}
                <br />
                {t("welcome.secretDecisionsTail")}
              </p>
              <div className="house-procession">
                {Object.keys(c.houses)
                  .filter((h) => a.units[h])
                  .map((h) => (
                    <Crest key={h} house={h} size={46} />
                  ))}
              </div>
              <div className="welcome-details">
                <span>
                  <Shield size={16} />
                  {t("welcome.noRegistration")}
                </span>
                <span>
                  <Swords size={16} />
                  {t("players.title")}
                </span>
                <span>
                  <Clock size={16} />
                  {t("welcome.yourPace")}
                </span>
              </div>
              <small>{t("rules.inDevelopment")}</small>
            </div>
            <form
              className="create-room"
              onSubmit={(e) => {
                e.preventDefault();
                void run(async () =>
                  enter(
                    await api<Room>(
                      roomID
                        ? `/api/rooms/${encodeURIComponent(roomID)}/join`
                        : "/api/rooms",
                      roomID
                        ? { name, house: chosenHouse }
                        : { name, title, settings, house: chosenHouse },
                    ),
                  ),
                );
              }}
            >
              <div className="form-heading">
                <span className="eyebrow">
                  {roomID
                    ? t("room.invitationHeading")
                    : t("room.newTableHeading")}
                </span>
                <Crown />
              </div>
              <h2>{roomID ? t("room.takeSeat") : t("room.create")}</h2>
              <label>
                {t("players.name")}
                <input
                  value={name}
                  maxLength={40}
                  autoComplete="nickname"
                  required
                  placeholder={t("players.namePlaceholder")}
                  onChange={(e) => setName(e.target.value)}
                />
              </label>
              <HousePicker
                value={chosenHouse}
                onChange={setChosenHouse}
                disabled={busy || (!!roomID && invitation?.id !== roomID)}
                houses={Object.keys(c.houses)
                  .filter((house) => a.units[house])
                  .filter(
                    (house) =>
                      roomID ||
                      c.baseSetups
                        .find(
                          (setup) =>
                            setup.playerCount === (settings.playerCount || 6),
                        )
                        ?.houses.includes(house),
                  )
                  .map((id) => ({
                    id,
                    name: c.houseNames[id],
                    available:
                      !roomID ||
                      !!(
                        invitation?.id === roomID &&
                        invitation.seats.find((seat) => seat.house === id)
                          ?.available
                      ),
                  }))}
              />
              {!!roomID &&
                invitation?.id === roomID &&
                invitation.phase !== "lobby" && (
                  <p role="status">{t("lobby.started")}</p>
                )}
              {!roomID && (
                <>
                  <label>
                    {t("room.title")}
                    <input
                      value={title}
                      maxLength={40}
                      onChange={(e) => setTitle(e.target.value)}
                    />
                  </label>
                  <div className="scenario-list">
                    {expansions.map((e) => (
                      <div
                        className={`scenario ${e.available ? "selected" : "locked"}`}
                        key={e.id}
                      >
                        <span className="scenario-check">
                          {e.available ? (
                            <Check size={14} />
                          ) : (
                            <Shield size={13} />
                          )}
                        </span>
                        <div>
                          <b>{e.title}</b>
                          <small>
                            {e.available
                              ? e.description
                              : t("expansions.comingLater")}
                          </small>
                        </div>
                      </div>
                    ))}
                  </div>
                  <RoomSettings
                    value={settings}
                    onChange={(next) => {
                      setSettings(next);
                      if (
                        !c.baseSetups
                          .find(
                            (setup) =>
                              setup.playerCount === (next.playerCount || 6),
                          )
                          ?.houses.includes(chosenHouse)
                      )
                        setChosenHouse("stark");
                    }}
                  />
                </>
              )}
              <label>
                {t("room.code")}
                <span className="subtle">{t("room.invitedHint")}</span>
                <input
                  value={roomID}
                  maxLength={16}
                  placeholder={t("room.codePlaceholder")}
                  onChange={(e) =>
                    setRoomID(e.target.value.trim().toLowerCase())
                  }
                />
              </label>
              <button
                className="primary large"
                disabled={
                  busy ||
                  !name.trim() ||
                  !chosenHouse ||
                  (!!roomID &&
                    (invitation?.id !== roomID || invitation.phase !== "lobby"))
                }
              >
                {roomID ? t("room.join") : t("room.gatherCouncil")}
                <ArrowRight size={18} />
              </button>
              <small>{t("room.cookieNotice")}</small>
            </form>
          </main>
        ) : (
          <main className="game-workspace">
            <div className="hud-panel-buttons">
              <button
                aria-label={t("panels.housesLabel")}
                aria-pressed={housesOpen}
                onClick={() => {
                  setHousesOpen(!housesOpen);
                  if (
                    !housesOpen &&
                    matchMedia("(max-width: 1150px)").matches
                  ) {
                    setInspectorOpen(false);
                    setTracksOpen(false);
                  }
                }}
              >
                <Crown size={16} />
                {t("panels.houses")}
              </button>
              <button
                aria-label={t("panels.areaLabel")}
                aria-pressed={inspectorOpen}
                onClick={() => {
                  setInspectorOpen(!inspectorOpen);
                  if (
                    !inspectorOpen &&
                    matchMedia("(max-width: 1150px)").matches
                  )
                    setHousesOpen(false);
                  if (!inspectorOpen) setTracksOpen(false);
                }}
              >
                <Flag size={16} />
                {t("panels.area")}
              </button>
            </div>
            <button
              className="influence-toggle"
              aria-label={t("ui.toggleInfluence")}
              aria-pressed={tracksOpen}
              onClick={() => {
                setTracksOpen(!tracksOpen);
                setInspectorOpen(tracksOpen);
                if (!tracksOpen && matchMedia("(max-width: 1150px)").matches)
                  setHousesOpen(false);
              }}
            >
              <Crown size={16} />
              {t("ui.influence")}
            </button>
            {tracksOpen && (
              <InfluencePanel
                room={room}
                catalog={c}
                onClose={() => {
                  setTracksOpen(false);
                  setInspectorOpen(true);
                }}
              />
            )}
            <aside className="houses" data-tour="houses">
              <div className="room-caption">
                <span className="eyebrow" title={room.name}>
                  {room.name}
                </span>
                <button
                  title={t("room.copyInvitation")}
                  onClick={() => {
                    void navigator.clipboard
                      .writeText(location.href)
                      .then(() => {
                        setCopied(true);
                        setTimeout(() => setCopied(false), 2000);
                      })
                      .catch(() => setError(t("room.copyAddressFallback")));
                  }}
                >
                  {copied ? <Check size={15} /> : <Copy size={15} />}
                </button>
              </div>
              <div className="my-house">
                <Crest house={room.yourHouse} size={57} />
                <div>
                  <small>{t("hud.yourHouse")}</small>
                  <h2 title={c.houseNames[room.yourHouse]}>
                    {c.houseNames[room.yourHouse]}
                  </h2>
                  <span title={me?.name}>{me?.name}</span>
                </div>
              </div>
              {room.phase === "lobby" && (
                <HousePicker
                  value={room.yourHouse}
                  disabled={busy}
                  onChange={(house) => {
                    setInspectedCard(undefined);
                    send({ type: "choose_house", house });
                  }}
                  houses={room.seats.map((seat) => ({
                    id: seat.house,
                    name: c.houseNames[seat.house],
                    available: seat.you || seat.bot || !seat.occupied,
                  }))}
                />
              )}
              <div className="house-resources">
                <div>
                  <b>{room.match?.power[room.yourHouse] ?? 5}</b>
                  <small>{t("resources.power")}</small>
                </div>
                <div>
                  <b>
                    {room.match?.supply[room.yourHouse] ??
                      c.houses[room.yourHouse].supplyLevel}
                  </b>
                  <small>{t("resources.supply")}</small>
                </div>
                <div>
                  <b>
                    {castleCount(room, room.yourHouse, c)}
                    <i>/7</i>
                  </b>
                  <small>{t("resources.castles")}</small>
                </div>
              </div>
              <div className="section-heading">
                <h3>{t("houses.title")}</h3>
                <small>
                  {t("game.roundPrefix")}
                  {room.round}/10
                </small>
              </div>
              <div className="seat-list">
                {room.seats.map((seat) => (
                  <HouseSeat
                    key={seat.house}
                    seat={seat}
                    room={room}
                    catalog={c}
                    art={art}
                    busy={busy}
                    onConfigure={(bot, difficulty) =>
                      send({
                        type: "configure",
                        house: seat.house,
                        bot,
                        difficulty,
                      })
                    }
                  />
                ))}
              </div>
              {room.match && (
                <details className="tracks">
                  <summary>{t("tracks.title")}</summary>
                  {Object.entries(room.match.tracks).map(([track, houses]) => (
                    <div key={track}>
                      <small>
                        {
                          (
                            {
                              ironThrone: t("tracks.ironThrone"),
                              fiefdoms: t("tracks.fiefdoms"),
                              kingsCourt: t("tracks.kingsCourt"),
                            } as Record<string, string>
                          )[track]
                        }
                      </small>
                      <div>
                        {houses.map((h, i) => (
                          <span key={h} title={`${i + 1}. ${c.houseNames[h]}`}>
                            <Crest house={h} size={20} />
                            <i>{i + 1}</i>
                          </span>
                        ))}
                      </div>
                    </div>
                  ))}
                </details>
              )}
              <details className="event-log">
                <summary>{t("game.chronicle")}</summary>
                <ol className="events">
                  {room.log
                    .slice(-15)
                    .reverse()
                    .map((line, i) => (
                      <li key={i}>{line}</li>
                    ))}
                </ol>
              </details>
              {!!room.match?.combats?.length && (
                <details className="combat-history">
                  <summary>
                    {t("combat.history")} · {room.match.combats.length}
                  </summary>
                  <ul>
                    {[...room.match.combats].reverse().map((record) => (
                      <li key={record.id}>
                        <button onClick={() => combatPresentation.show(record)}>
                          <small>
                            {t("combat.attackAnnouncement", {
                              p0: record.round,
                              p1: record.id,
                            })}
                          </small>
                          <span>
                            {regionName(
                              c.board.regions.find(
                                (region) => region.id === record.from,
                              ),
                            )}{" "}
                            →{" "}
                            {regionName(
                              c.board.regions.find(
                                (region) => region.id === record.to,
                              ),
                            )}
                          </span>
                          <small>
                            {record.result
                              ? `${record.result.attack} : ${record.result.defense} · ${c.houseNames[record.result.winner]}`
                              : t("combat.pendingResult")}
                          </small>
                        </button>
                      </li>
                    ))}
                  </ul>
                </details>
              )}
            </aside>
            <div className="table-center">
              <div
                className={`phase-banner ${myTurn || ownRetreat ? "your-turn" : ""}`}
                data-tour="phase"
              >
                <div>
                  <span className="eyebrow">
                    {t("hud.roundPrefix")}
                    {room.round} / 10
                  </span>
                  <h2>{phases[room.phase] || room.phase}</h2>
                  <PhaseSteps phase={room.phase} />
                </div>
                <div className="phase-status">
                  {room.phase !== "lobby" && <GameClock room={room} />}
                  <span className="phase-context">
                    {room.phase === "planning" ? (
                      <>
                        <Shield size={17} />
                        <span>
                          {t("planning.yourOrders")}
                          {planned} / {myRegions.length}
                        </span>
                      </>
                    ) : room.phase === "retreat" && battle?.loser ? (
                      <>
                        <Crest
                          house={battle.retreatChooser || battle.loser}
                          size={27}
                        />
                        <span>
                          {ownRetreat
                            ? battle.loser !== room.yourHouse
                              ? t("retreat.chooseEnemyRobb")
                              : t("retreat.chooseOnMap")
                            : t("retreat.choosingHouse", {
                                p0: c.houseNames[
                                  battle.retreatChooser || battle.loser
                                ],
                              })}
                        </span>
                      </>
                    ) : room.match?.activeHouse ? (
                      <>
                        <Crest house={room.match.activeHouse} size={27} />
                        <span>
                          {myTurn
                            ? t("turn.yours")
                            : t("turn.houseTurn", {
                                p0: c.houseNames[room.match.activeHouse],
                              })}
                        </span>
                      </>
                    ) : (
                      <span>
                        {room.phase === "lobby"
                          ? t("room.configurePlayers")
                          : room.phase === "finished"
                            ? t("game.finished")
                            : t("planning.allRevealed")}
                      </span>
                    )}
                  </span>
                </div>
              </div>
              {gameManifest ? (
                <Suspense
                  fallback={
                    <div className="game-board">{t("loading.table3d")}</div>
                  }
                >
                  <GameBoard3D
                    key={room.id}
                    manifest={gameManifest}
                    catalog={c}
                    room={room}
                    assets={a}
                    selected={selected}
                    target={target}
                    onSelect={select}
                    onTarget={moving && actionAvailable ? setTarget : undefined}
                    motion={motion}
                    onRetry={() => location.reload()}
                    retreat={
                      ownRetreat && battle
                        ? {
                            from: battle.to,
                            targets: battle.retreats,
                            target: retreatTarget,
                            busy,
                            onTarget: setRetreatTarget,
                            onConfirm: () => {
                              if (battle.retreats.includes(retreatTarget))
                                send({
                                  type: "retreat",
                                  target: retreatTarget,
                                });
                            },
                          }
                        : undefined
                    }
                    action={mapAction}
                    frameRequest={frameRequest}
                    combatPlayback={
                      combatPresentation.playback
                        ? {
                            ...combatPresentation.playback,
                            motion: animateCombat,
                            onComplete: combatPresentation.completePlayback,
                          }
                        : undefined
                    }
                    paused={
                      !!flow.active ||
                      combatPresentation.visible ||
                      settingsOpen ||
                      !!inspectedCard ||
                      tour
                    }
                    battleFocus={
                      combatPresentation.playback
                        ? {
                            from: combatPresentation.playback.record.from,
                            to: combatPresentation.playback.record.to,
                            house: combatPresentation.playback.record.attacker,
                          }
                        : undefined
                    }
                  />
                </Suspense>
              ) : (
                <section
                  className="game-board game3d-failure game3d-unavailable"
                  role="status"
                >
                  <Shield />
                  <h2>
                    {gameAssetsLoading
                      ? t("loading.preparingTable")
                      : t("assets.required")}
                  </h2>
                  <p>{gameAssetsError || t("loading.definitions")}</p>
                  {!gameAssetsLoading && (
                    <>
                      <code>node scripts/import-3d-assets.mjs</code>
                      <small>{t("assets.sourceLocationHint")}</small>
                      <button onClick={() => void loadGameAssets()}>
                        {t("assets.retryLoading")}
                      </button>
                    </>
                  )}
                </section>
              )}
              <div className="action-dock" data-tour="actions">
                <div>
                  <small>
                    {ravenActive
                      ? t("raven.title")
                      : provisionActive
                        ? t("provision.open")
                        : eventsActive
                          ? et("open")
                          : room.influence
                            ? t("influence.eyebrow")
                            : room.phase === "lobby"
                              ? t("room.preparationHeading")
                              : room.phase === "planning"
                                ? t("welcome.secretDecisionsHeading")
                                : ownRetreat ||
                                    (room.phase !== "retreat" && myTurn)
                                  ? t("hud.yourTurn")
                                  : t("hud.gameProgress")}
                  </small>
                  <p>
                    {ravenActive
                      ? t("raven.actionHint")
                      : provisionActive
                        ? t("provision.actionHint")
                        : eventsActive
                          ? et("actionHint")
                          : room.influence
                            ? t("influence.actionHint")
                            : room.phase === "westeros"
                              ? t("influence.westerosPending")
                              : room.phase === "lobby"
                                ? t("room.inviteFriendsHint")
                                : room.phase === "planning"
                                  ? me?.ready
                                    ? t("planning.waitingForOthers")
                                    : t("planning.assignOrdersHint")
                                  : room.phase === "orders_revealed"
                                    ? t("planning.continueHint")
                                    : room.phase === "finished"
                                      ? t("victory.houseWins", {
                                          p0: c.houseNames[
                                            room.match?.winner || ""
                                          ],
                                        })
                                      : room.phase === "retreat" &&
                                          battle?.loser
                                        ? ownRetreat
                                          ? t("retreat.confirmHint")
                                          : t("retreat.waitingForHouse", {
                                              p0: c.houseNames[battle.loser],
                                            })
                                        : myTurn
                                          ? t("ability.chooseOrderHint")
                                          : t("ability.waitingForHouse", {
                                              p0:
                                                c.houseNames[
                                                  room.match?.activeHouse || ""
                                                ] || t("welcome.playerWord"),
                                            })}
                  </p>
                </div>
                {ravenActive && (
                  <button
                    className="primary"
                    onClick={() => {
                      setRavenOpen(true);
                      setShowResult(false);
                      setShowInfluenceResult(false);
                      setInspectedCard(undefined);
                    }}
                  >
                    <Feather size={17} />
                    {t("raven.title")}
                  </button>
                )}
                {room.influence && (
                  <button
                    className="primary"
                    onClick={() => setInfluenceOpen(true)}
                  >
                    <Crown size={17} />
                    {t("influence.openAuction")}
                  </button>
                )}
                {(eventsActive || room.westeros?.current) && (
                  <button
                    className={eventsActive ? "primary" : "subtle-button"}
                    onClick={() => {
                      setEventsOpen(true);
                      setShowResult(false);
                      setShowInfluenceResult(false);
                      setInspectedCard(undefined);
                    }}
                  >
                    <Shield size={17} />
                    {et(eventsActive ? "open" : "history")}
                  </button>
                )}
                {provisionActive && (
                  <button
                    className="primary"
                    onClick={() => setProvisionOpen(true)}
                  >
                    <Shield size={17} />
                    {t("provision.open")}
                  </button>
                )}
                {mapAction && !inspectorOpen && (
                  <button
                    className="primary"
                    onClick={() => setInspectorOpen(true)}
                  >
                    {t("actions.openPanel")}
                  </button>
                )}
                {!!room.match?.lastInfluence?.length && !room.influence && (
                  <button
                    className="subtle-button"
                    onClick={() => setShowInfluenceResult(true)}
                  >
                    <Crown size={17} />
                    {t("influence.openResults")}
                  </button>
                )}
                {room.phase === "lobby" && room.owner && (
                  <button
                    className="primary"
                    disabled={busy || room.seats.some((s) => !s.occupied)}
                    onClick={() => send({ type: "start" })}
                  >
                    <Play size={17} />
                    {t("game.start")}
                  </button>
                )}
                {room.phase === "planning" && (
                  <button
                    className="primary"
                    disabled={busy}
                    onClick={() =>
                      send({ type: me?.ready ? "unready" : "ready" })
                    }
                  >
                    {me?.ready ? <Pause size={17} /> : <Check size={17} />}{" "}
                    {me?.ready
                      ? t("planning.changeOrders")
                      : t("planning.ready")}
                  </button>
                )}
                {room.phase === "orders_revealed" && (
                  <button
                    className="primary"
                    disabled={busy}
                    onClick={() => send({ type: "continue" })}
                  >
                    {t("planning.resolveOrders")}
                    <ArrowRight size={17} />
                  </button>
                )}
                {room.match?.lastCombat && (
                  <button
                    className="subtle-button"
                    onClick={() => {
                      const record = room.match?.combats?.at(-1);
                      if (record?.result)
                        combatPresentation.show(record, "result");
                      else setShowResult(true);
                    }}
                  >
                    <Swords size={16} />
                    {t("combat.title")}
                  </button>
                )}
              </div>
            </div>
            <aside className="inspector" data-tour="inspector">
              <button
                className="hud-close-panel"
                aria-label={t("panels.hideArea")}
                onClick={() => setInspectorOpen(false)}
              >
                <X size={17} />
              </button>
              <div className="area-heading">
                <span className="eyebrow">{t("hud.selectedArea")}</span>
                <h2>{regionName(region)}</h2>
                <p className="subtle">
                  {region?.type === "land"
                    ? t("terrain.land")
                    : region?.type === "sea"
                      ? t("terrain.sea")
                      : t("terrain.port")}
                  {room.match?.control[selected]
                    ? ` · ${c.houseNames[room.match.control[selected]]}`
                    : ""}
                </p>
              </div>
              {room.fortifications?.[selected] && (
                <FortificationInfo value={room.fortifications[selected]} />
              )}
              {mapAction ? (
                <MapActionPanel
                  action={mapAction}
                  room={room}
                  catalog={c}
                  assets={a}
                  selected={selected}
                  target={target}
                  onTarget={setTarget}
                  onFrame={() => setFrameRequest((value) => value + 1)}
                  leaveToken={leaveToken}
                  onLeaveToken={setLeaveToken}
                />
              ) : (
                <>
                  <div className="stats">
                    <div>
                      <b>{region?.supplyIcons || 0}</b>
                      <span>{t("resources.supply")}</span>
                    </div>
                    <div>
                      <b>{region?.crownIcons || 0}</b>
                      <span>{t("resources.power")}</span>
                    </div>
                    <div>
                      <b>{region?.castleLevel || 0}</b>
                      <span>{t("terrain.castle")}</span>
                    </div>
                  </div>
                  <h3>{t("units.title")}</h3>
                  {army.length ? (
                    army.map((u) => (
                      <div key={u.unitType} className="unit detailed-unit">
                        <img src={a.units[u.house]?.[u.unitType]} alt="" />
                        <div>
                          <b>{unitNames[u.unitType]}</b>
                          <small>{c.houseNames[u.house]}</small>
                        </div>
                        <strong>{u.quantity}</strong>
                      </div>
                    ))
                  ) : (
                    <p className="subtle">{t("board.noUnits")}</p>
                  )}
                  <div className="section-heading">
                    <h3>{t("orders.title")}</h3>
                    {order && <span>{order.name}</span>}
                  </div>
                  {ours && room.phase === "planning" && room.orderBudget && (
                    <OrderBudget value={room.orderBudget} />
                  )}
                  {ours && room.phase === "planning" ? (
                    <OrderPalette
                      key={selected}
                      orders={c.orders}
                      available={room.availableOrders?.[selected] || []}
                      selected={order?.id}
                      used={Object.entries(room.orders)
                        .filter(([id]) => id !== selected)
                        .map(([, order]) => order)}
                      locked={
                        busy
                          ? t("common.sendingChange")
                          : me?.ready
                            ? t("planning.changeOrdersFirst")
                            : undefined
                      }
                      onChoose={(id) =>
                        send({ type: "order", region: selected, order: id })
                      }
                    />
                  ) : (
                    <div className="selected-order">
                      {order ? (
                        <>
                          <OrderToken order={order} decorative />
                          <span>{order.name}</span>
                        </>
                      ) : (
                        <span>
                          {room.phase === "planning" && !ours
                            ? t("orders.hidden")
                            : t("orders.none")}
                        </span>
                      )}
                    </div>
                  )}
                  {order && !(ours && room.phase === "planning") && (
                    <OrderDescription order={order} />
                  )}
                </>
              )}
            </aside>
            <HouseHand
              room={room}
              cards={c.houses[room.yourHouse].houseCards}
              houseName={c.houseNames[room.yourHouse]}
              color={c.houses[room.yourHouse].color}
              art={(id) => art(id, room.yourHouse)}
              motion={motion}
              open={handOpen}
              onToggle={setHandOpen}
              onInspect={setInspectedCard}
            />
          </main>
        )}
        {room && combatPresentation.onMap && combatPresentation.playback && (
          <PlaybackControls
            value={combatPresentation.playback}
            catalog={c}
            motion={animateCombat}
            onPause={combatPresentation.togglePlayback}
            onSkip={combatPresentation.skipPlayback}
            onContinue={combatPresentation.resume}
            onDecision={
              hasCombatDecision(room)
                ? combatPresentation.toDecision
                : undefined
            }
          />
        )}
        {toast && (
          <div className="game-toast" role="status">
            <Flag size={17} />
            {toast}
          </div>
        )}
        {dialogRoom && eventsOpen && (
          <EventDialog
            key={eventsIdentity(dialogRoom) || `${dialogRoom.id}:history`}
            room={dialogRoom}
            catalog={c}
            busy={busy || flow.stale}
            error={error}
            onCommand={sendDecision}
            art={art}
            onClose={() => setEventsOpen(false)}
            onShowRegion={(id) => {
              select(id);
              setEventsOpen(false);
              setTab("board");
            }}
          />
        )}
        {dialogRoom?.raven && ravenOpen && (
          <RavenDialog
            key={ravenIdentity(dialogRoom)}
            room={dialogRoom}
            catalog={c}
            busy={busy || flow.stale}
            error={error}
            onCommand={sendDecision}
            onClose={() => setRavenOpen(false)}
            onShowRegion={(id) => {
              select(id);
              setRavenOpen(false);
              setTab("board");
            }}
          />
        )}
        {dialogRoom?.influence && influenceOpen && (
          <InfluenceDialog
            key={`${dialogRoom.id}:${dialogRoom.round}:${dialogRoom.influence.track}:${dialogRoom.influence.stage}`}
            room={dialogRoom}
            catalog={c}
            busy={busy || flow.stale}
            error={error}
            onCommand={sendDecision}
            onClose={() => setInfluenceOpen(false)}
          />
        )}
        {dialogRoom && provisionOpen && (
          <ProvisionDialog
            key={`${dialogRoom.id}:${dialogRoom.round}:${dialogRoom.phase}:${dialogRoom.match?.activeHouse}`}
            room={dialogRoom}
            catalog={c}
            assets={a}
            busy={busy || flow.stale}
            error={error}
            onCommand={sendDecision}
            onClose={() => setProvisionOpen(false)}
            onShowRegion={(id) => {
              select(id);
              setProvisionOpen(false);
              setTab("board");
            }}
          />
        )}
        {dialogRoom &&
          influenceOpen &&
          !dialogRoom.influence &&
          !!dialogRoom.match?.lastInfluence?.length && (
            <InfluenceReport
              room={dialogRoom}
              catalog={c}
              onClose={flow.close}
            />
          )}
        {room &&
          !room.influence &&
          !provisionActive &&
          !eventsOpen &&
          !ravenOpen &&
          !flow.active &&
          showInfluenceResult &&
          !!room.match?.lastInfluence?.length && (
            <InfluenceReport
              room={room}
              catalog={c}
              onClose={() => setShowInfluenceResult(false)}
            />
          )}
        {room && inspectedCard && !flow.active && (
          <CardInspector
            cards={c.houses[room.yourHouse].houseCards}
            selected={inspectedCard}
            houseName={c.houseNames[room.yourHouse]}
            color={c.houses[room.yourHouse].color}
            art={(id) => art(id, room.yourHouse)}
            motion={motion}
            onSelect={setInspectedCard}
            onClose={() => setInspectedCard(undefined)}
          />
        )}
        {room && settingsOpen && !flow.active && (
          <div className="modal-backdrop">
            <section
              className="settings-modal"
              role="dialog"
              aria-modal="true"
              aria-labelledby="settings-title"
            >
              <div className="modal-top">
                <h2 id="settings-title">{t("room.settingsTitle")}</h2>
                <button
                  aria-label={t("common.close")}
                  onClick={() => setSettingsOpen(false)}
                >
                  <X size={18} />
                </button>
              </div>
              <RoomSettings
                value={settings}
                onChange={setSettings}
                disabled={!room.owner || room.phase !== "lobby"}
              />
              {error && (
                <p className="settings-error" role="alert">
                  {error}
                </p>
              )}
              <p className="subtle">{t("room.settingsHint")}</p>
              <div className="expansion-notice">
                <Shield size={20} />
                <p>{t("expansions.unavailableHint")}</p>
              </div>
              {room.owner && room.phase === "lobby" && (
                <button
                  className="primary"
                  disabled={busy}
                  onClick={() => {
                    send({ type: "settings", settings }, () =>
                      setSettingsOpen(false),
                    );
                  }}
                >
                  {t("room.saveSettings")}
                </button>
              )}
            </section>
          </div>
        )}
        {room &&
          cardChoice &&
          room.phase === "card_effect" &&
          cardChoice.house === room.yourHouse &&
          !showResult &&
          !showInfluenceResult &&
          !combatPresentation.visible &&
          !combatPresentation.onMap &&
          !settingsOpen &&
          !inspectedCard &&
          !tour &&
          !flow.active && (
            <div className="modal-backdrop">
              <section
                className="battle-modal card-effect-modal"
                ref={effectDialog}
                onKeyDown={(event) => {
                  if (event.key !== "Tab") return;
                  const buttons = Array.from(
                    event.currentTarget.querySelectorAll<HTMLButtonElement>(
                      "button:not(:disabled)",
                    ),
                  );
                  const next = event.shiftKey ? buttons.at(-1) : buttons[0];
                  if (
                    document.activeElement ===
                    (event.shiftKey ? buttons[0] : buttons.at(-1))
                  ) {
                    event.preventDefault();
                    next?.focus();
                  }
                }}
                role="dialog"
                aria-modal="true"
                aria-labelledby="effect-title"
                data-testid="card-effect-choice"
              >
                <div className="modal-top">
                  <p className="eyebrow">
                    {isTidesChoice || isBladeChoice
                      ? t("tides.bladeTitle")
                      : t("ability.yourDecisionHeading")}
                  </p>
                  <button
                    type="button"
                    data-testid="choice-language-switch"
                    aria-label={t("settings.changeLanguage")}
                    onClick={() => setLanguage(language === "ru" ? "en" : "ru")}
                  >
                    {language.toUpperCase()}
                  </button>
                </div>
                <h2 id="effect-title">
                  {isBladeChoice
                    ? t("tides.bladeTitle")
                    : isTidesChoice
                      ? t("tides.title")
                      : choiceCard?.name || t("ability.choiceTitle")}
                </h2>
                <p>
                  {isBladeChoice
                    ? t("tides.bladeHint")
                    : isTidesChoice
                      ? cardChoice.kind === CardChoiceKind.TidesRedraw
                        ? t("tides.redrawHint")
                        : t("tides.bladeHint")
                      : cardChoice.kind === CardChoiceKind.Replace
                        ? t("combat.cardCancelledHint")
                        : choiceCard?.description}
                </p>
                {isTidesChoice && battle && (
                  <div className="tides-pair">
                    {[battle.attacker, battle.defender].map((house) => (
                      <TidesCard
                        key={house}
                        card={battle.modifiers?.[house]}
                        houseName={c.houseNames[house]}
                      />
                    ))}
                  </div>
                )}
                <div className="card-effect-options">
                  {(cardChoice.options || []).map((value) => (
                    <button
                      key={value}
                      disabled={busy}
                      onClick={() => send({ type: "ability", choice: value })}
                    >
                      {choiceLabel(value)}
                    </button>
                  ))}
                </div>
                {cardChoice.optional && (
                  <button
                    className="secondary"
                    disabled={busy}
                    onClick={() => send({ type: "ability", choice: "" })}
                  >
                    {t("ability.skip")}
                  </button>
                )}
                <p className="subtle">{t("ability.savedDecisionHint")}</p>
              </section>
            </div>
          )}
        {room &&
          battle &&
          inBattle &&
          room.phase === "combat" &&
          !showResult &&
          !showInfluenceResult &&
          !flow.active &&
          !combatPresentation.visible &&
          !combatPresentation.onMap &&
          !settingsOpen &&
          !inspectedCard &&
          !tour && (
            <Modal
              titleID="battle-title"
              onClose={() => {}}
              className="combat-card-choice"
            >
              <span className="eyebrow">{t("combat.secretCardHeading")}</span>
              <h2 id="battle-title">
                {regionName(c.board.regions.find((r) => r.id === battle.to))}
              </h2>
              <div className="combatants">
                <div>
                  <Crest house={battle.attacker} size={70} />
                  <h3>{c.houseNames[battle.attacker]}</h3>
                  <span>{t("combat.attacker")}</span>
                  <small>
                    {battle.chosen[battle.attacker]
                      ? t("combat.cardChosen")
                      : t("combat.choosingCard")}
                  </small>
                </div>
                <Swords size={40} />
                <div>
                  <Crest house={battle.defender} size={70} />
                  <h3>{c.houseNames[battle.defender]}</h3>
                  <span>{t("combat.defender")}</span>
                  <small>
                    {battle.chosen[battle.defender]
                      ? t("combat.cardChosen")
                      : t("combat.choosingCard")}
                  </small>
                </div>
              </div>
              {battle.strength && (
                <details className="combat-breakdown-details">
                  <summary>{t("combat.breakdown")}</summary>
                  <StrengthBreakdown
                    value={battle.strength}
                    catalog={c}
                    assets={a}
                  />
                </details>
              )}
              {
                <>
                  <p>
                    {battle.yourCard
                      ? t("combat.cardLocked")
                      : t("combat.chooseCardHint")}
                  </p>
                  <div className="battle-hand">
                    {(room.availableCards || []).map((id) => {
                      const card = c.houses[room.yourHouse].houseCards[id];
                      return (
                        <button
                          key={id}
                          disabled={busy || !!battle.yourCard}
                          className={battle.yourCard === id ? "chosen" : ""}
                          onClick={() => send({ type: "card", card: id })}
                        >
                          <HouseCard
                            card={card}
                            src={art(id, room.yourHouse)}
                            color={c.houses[room.yourHouse].color}
                            motion={motion}
                          />
                          <span>{card.name}</span>
                          <small className="battle-card-description">
                            {card.description}
                          </small>
                        </button>
                      );
                    })}
                  </div>
                  <small className="combat-limit">
                    {t("ability.resolutionHint")}
                  </small>
                </>
              }
            </Modal>
          )}
        {room &&
          showResult &&
          !flow.active &&
          room.phase !== "card_effect" &&
          !room.influence &&
          !eventsOpen &&
          !eventsActive &&
          !ravenActive &&
          room.match?.lastCombat && (
            <CombatReport
              result={room.match.lastCombat}
              catalog={c}
              art={art}
              motion={motion}
              assets={a}
              onClose={() => setShowResult(false)}
            />
          )}
        {room &&
          combatPresentation.visible &&
          combatPresentation.active &&
          !showResult &&
          !showInfluenceResult &&
          !flow.active &&
          !settingsOpen &&
          !inspectedCard &&
          !tour &&
          (combatPresentation.active.stage === "intro" ? (
            <CombatIntro
              record={combatPresentation.active.record}
              catalog={c}
              assets={a}
              motion={animateCombat}
              approachViewed={combatPresentation.approachViewed}
              onNext={combatPresentation.next}
              onMap={combatPresentation.showMap}
              onDecision={
                hasCombatDecision(room)
                  ? combatPresentation.toDecision
                  : undefined
              }
            />
          ) : (
            combatPresentation.active.record.result && (
              <CombatReport
                key={combatPresentation.active.record.id}
                result={combatPresentation.active.record.result}
                record={combatPresentation.active.record}
                catalog={c}
                assets={a}
                art={art}
                motion={animateCombat}
                stage={combatPresentation.reportStage}
                onStage={combatPresentation.setReportStage}
                onClose={combatPresentation.next}
                onMap={combatPresentation.showMap}
                onDecision={
                  hasCombatDecision(room)
                    ? combatPresentation.toDecision
                    : undefined
                }
              />
            )
          ))}
        {room?.phase === "finished" && (
          <div className="winner-ribbon">
            <Crown size={32} />
            <span>
              {t("victory.ironThroneWinner")}{" "}
              <b>{c.houseNames[room.match?.winner || ""]}</b>
            </span>
          </div>
        )}
        {tour && !flow.active && (
          <Tutorial
            onClose={() => {
              setTour(false);
              localStorage.setItem("westeros-tour-v2", "true");
            }}
          />
        )}
      </div>
    </DecisionFlowContext.Provider>
  );
}
function ScrollIcon() {
  return <HelpCircle size={24} />;
}
