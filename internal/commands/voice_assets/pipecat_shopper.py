#!/usr/bin/env python3
from __future__ import annotations

import argparse
import asyncio
import contextlib
import json
import os
import select
import sys
import time
from dataclasses import dataclass

try:
    import termios
    import tty
except ImportError:  # pragma: no cover
    termios = None
    tty = None

from loguru import logger
from mcp import StdioServerParameters
from pipecat.frames.frames import (
    BotStartedSpeakingFrame,
    BotStoppedSpeakingFrame,
    FunctionCallInProgressFrame,
    FunctionCallResultFrame,
    FunctionCallsStartedFrame,
    InterruptionFrame,
    LLMContextFrame,
    TTSTextFrame,
    TranscriptionFrame,
    UserStartedSpeakingFrame,
    UserStoppedSpeakingFrame,
)
from pipecat.observers.base_observer import BaseObserver, FramePushed
from pipecat.pipeline.pipeline import Pipeline
from pipecat.pipeline.runner import PipelineRunner
from pipecat.pipeline.task import PipelineParams, PipelineTask
from pipecat.processors.aggregators.llm_context import LLMContext
from pipecat.processors.aggregators.llm_response_universal import LLMAssistantAggregator
from pipecat.services.mcp_service import MCPClient
from pipecat.services.openai.realtime import events
from pipecat.services.openai.realtime.llm import OpenAIRealtimeLLMService
from pipecat.transports.local.audio import LocalAudioTransport, LocalAudioTransportParams

SYSTEM_PROMPT = """
Jesteś MartMart Sous-Chef — polskojęzycznym agentem głosowym do planowania zakupów spożywczych.

Twoja rola:
- pomagasz użytkownikowi wymyślić coś fajnego do jedzenia, kiedy ma pustą lub nudną lodówkę,
- rozpoznajesz na co użytkownik ma ochotę, ile ma czasu, jaki ma budżet i co już ma w domu,
- proponujesz 2-4 sensowne kierunki posiłku albo prosty przepis,
- zamieniasz to na praktyczną listę zakupową,
- korzystasz z narzędzi MCP MartMart do wyszukiwania produktów, pracy z koszykiem, sesją i dostawą,
- pytasz proaktywnie o marki, promocje, zamienniki i produkty komplementarne.

Zasady prowadzenia rozmowy:
- mów po polsku, naturalnie i bardzo zwięźle,
- domyślnie odpowiadaj 1-2 krótkimi zdaniami; jeśli wystarczy, zadawaj jedno krótkie pytanie,
- nie rozwlekaj się, nie rób długich wstępów i nie czytaj całych list produktów,
- na starcie zadawaj maksymalnie 2 krótkie pytania naraz,
- najpierw doprecyzuj smak/posiłek, to co jest w domu, czas, budżet i liczbę porcji,
- jeśli użytkownik nie wie czego chce, zaproponuj 2-3 różne opcje i pomóż wybrać,
- przy każdym realnym zakupie dopytaj o preferencje marek, budżet, promocje oraz ewentualne zamienniki,
- po dodaniu głównego produktu zaproponuj sensowne produkty komplementarne,
- kiedy zaczynasz coś sprawdzać, powiedz hasłowo co robisz, np. „szukam napojów”, „sprawdzam promocje na mleko”, „sprawdzam termin dostawy”,
- kiedy używasz narzędzi, streszczaj wynik zwykłym językiem zamiast czytać surowy JSON,
- po wyniku narzędzia mów maksymalnie 1-2 krótkie zdania i przechodź do decyzji,
- gdy widzisz kilka sensownych produktów, porównuj je krótko: cena, marka, gramatura, zamienniki,
- jeśli produkt jest niedostępny lub słaby, zaproponuj alternatywy i poproś użytkownika o decyzję zanim wybierzesz zamiennik,
- jeśli nie ma slotu dostawy, powiedz to jasno i zapytaj co robić dalej, np. inny dzień, inna godzina, ewentualnie inny sklep jeśli taki wariant jest dostępny,
- zanim zmodyfikujesz koszyk, upewnij się czego dokładnie chce użytkownik,
- po zmianie koszyka podsumuj co zostało dodane/usunięte i zapytaj co dalej.

Zasady używania narzędzi:
- jeśli pytanie dotyczy realnych produktów, cen, promocji, dostępności, koszyka albo dostawy, użyj narzędzi MartMart zamiast zgadywać,
- nie zakładaj z góry jednego sklepu albo providera,
- gdy trzeba wykonać realne działania zakupowe, najpierw sprawdź jakie providery i narzędzia są dostępne w aktualnej sesji MCP,
- wybieraj providera per request na podstawie intencji użytkownika, dostępności narzędzi i kontekstu rozmowy,
- jeśli użytkownik nie wskazał sklepu, możesz krótko zapytać o preferowany sklep albo sam dobrać provider po sprawdzeniu dostępnych opcji i jasno to zakomunikować,
- jeśli narzędzia wspierają parametr providera, przekazuj go jawnie w wywołaniu zamiast polegać na globalnym domyśle,
- przed wywołaniem narzędzia lub serii narzędzi powiedz krótko co sprawdzasz,
- po wyniku narzędzia podaj 1-2 bardzo krótkie zdania podsumowania i jeśli potrzeba zadaj pytanie decyzyjne,
- gdy pokazujesz opcje, ogranicz się do 2-3 najlepszych,
- gdy wynik wymaga wyboru użytkownika, nie podejmuj decyzji sam: zapytaj wprost, np. „nie ma orzeszków ziemnych, chcesz zamienić na włoskie?”
- jeśli narzędzie zwróci błąd autoryzacji, 401 albo Unauthorized, nie powtarzaj w kółko tego samego wywołania; powiedz jasno, że potrzebne jest logowanie i zapytaj użytkownika, czy uruchomić logowanie
- nie mów, że coś sprawdziłeś, znalazłeś albo porównałeś, jeśli naprawdę nie użyłeś do tego narzędzia

Priorytety zakupowe:
- szukaj najpierw sensownych i praktycznych opcji, nie tylko najtańszych,
- zwracaj uwagę na promocje i większe opakowania, jeśli pasują do celu użytkownika,
- proponuj dodatki tylko jeśli naprawdę pomagają: np. pieczywo, sos, ser, zioła, napój, deser,
- pilnuj, by lista miała sens jako cały posiłek.

Bezpieczeństwo i scope:
- możesz wyszukiwać produkty, oglądać koszyk, dodawać i usuwać po wyraźnym uzgodnieniu,
- możesz sprawdzać sloty dostawy,
- nie twierdzisz, że zamówienie zostało finalnie złożone ani opłacone,
- jeśli sesja nie działa, pomóż użytkownikowi zalogować się i dopiero wróć do zakupów.
""".strip()

KICKOFF_PROMPT = """
Rozpocznij rozmowę zakupową po polsku. Przywitaj się bardzo krótko i od razu przejdź do rzeczy.
W pierwszej odpowiedzi zadaj najwyżej 2 krótkie pytania: o to, na co mam ochotę, i co już mam w domu.
Wspomnij jednym krótkim zdaniem, że możesz też pomóc dobrać sklep lub providera, marki, promocje i zamienniki.
""".strip()

def parse_json_value(value: object) -> object:
    if isinstance(value, (dict, list)):
        return value
    if isinstance(value, str):
        stripped = value.strip()
        if not stripped:
            return value
        if stripped.startswith("{") or stripped.startswith("["):
            with contextlib.suppress(Exception):
                return json.loads(stripped)
    return value


def extract_http_error(payload: object) -> tuple[int | None, str | None]:
    normalized = parse_json_value(payload)
    if isinstance(normalized, dict):
        status = normalized.get("status")
        reason = normalized.get("reason")
        if isinstance(status, int):
            return status, str(reason) if reason else None
    return None, None


def localized_text(value: object) -> str:
    if isinstance(value, str):
        return value.strip()
    if isinstance(value, dict):
        for key in ("pl", "value", "name", "displayName", "title", "en"):
            inner = value.get(key)
            if isinstance(inner, str) and inner.strip():
                return inner.strip()
        for inner in value.values():
            if isinstance(inner, str) and inner.strip():
                return inner.strip()
    return ""


def money_text(value: object) -> str:
    if isinstance(value, (int, float)):
        return f"{float(value):.2f} zł"
    if isinstance(value, str):
        stripped = value.strip()
        if stripped:
            return stripped
    if isinstance(value, dict):
        for key in ("price", "gross", "amount", "value"):
            inner = value.get(key)
            if isinstance(inner, (int, float)):
                return f"{float(inner):.2f} zł"
            if isinstance(inner, str) and inner.strip():
                return inner.strip()
    return ""


def compact_text(value: object) -> str:
    if value is None:
        return ""
    if isinstance(value, float):
        return f"{value:g}"
    return str(value).strip()


def extract_products(payload: object) -> tuple[list[dict], dict]:
    normalized = parse_json_value(payload)
    if isinstance(normalized, dict):
        if isinstance(normalized.get("products"), list):
            return [p for p in normalized.get("products", []) if isinstance(p, dict)], normalized
        if isinstance(normalized.get("api_response"), dict):
            return extract_products(normalized["api_response"])
    return [], normalized if isinstance(normalized, dict) else {}


def summarize_product_entry(entry: dict) -> str:
    product = entry.get("product") if isinstance(entry.get("product"), dict) else entry
    pid = compact_text(entry.get("productId") or product.get("productId") or entry.get("id") or product.get("id"))
    name = localized_text(product.get("name") or product.get("displayName") or entry.get("name") or entry.get("displayName"))
    brand = compact_text(product.get("brand") or entry.get("brand"))
    price = money_text(product.get("price") or entry.get("price"))
    grammage = compact_text(product.get("grammage") or entry.get("grammage"))
    unit = compact_text(product.get("unitOfMeasure") or entry.get("unitOfMeasure"))
    available = product.get("isAvailable")

    main = name or pid or "produkt"
    parts = [main]
    if pid:
        parts.append(f"id={pid}")
    if brand:
        parts.append(f"marka={brand}")
    if price:
        parts.append(f"cena={price}")
    if grammage:
        gram_text = f"{grammage} {unit}".strip()
        parts.append(f"gramatura={gram_text}")
    if isinstance(available, bool):
        parts.append("dostępny" if available else "niedostępny")
    return " | ".join(parts)


def summarize_products_search_output(raw: object) -> str:
    payload = parse_json_value(raw)
    status, reason = extract_http_error(payload)
    if status is not None:
        return (
            f"BŁĄD AUTORYZACJI narzędzia products_search: HTTP {status} {reason or ''}. "
            "Sesja sklepu może być nieważna. Zapytaj użytkownika, czy uruchomić logowanie przez session_login."
        ).strip()

    products, root = extract_products(payload)
    total_count = root.get("totalCount") if isinstance(root, dict) else None
    total_label = compact_text(total_count) or str(len(products))
    if not products:
        return "products_search: brak wyników. Powiedz użytkownikowi, że nic sensownego nie znaleziono i zapytaj o inny wariant lub zamiennik."

    lines = [summarize_product_entry(product) for product in products[:5]]
    return (
        f"products_search: znaleziono {total_label} wyników. Najbardziej pasujące produkty:\n"
        + "\n".join(f"{idx + 1}. {line}" for idx, line in enumerate(lines))
        + "\nNa podstawie tej listy porównaj krótko opcje i zaproponuj konkretny wybór albo dopytaj o preferencje."
    )


def summarize_products_by_ids_output(raw: object) -> str:
    payload = parse_json_value(raw)
    status, reason = extract_http_error(payload)
    if status is not None:
        return f"products_by_ids: błąd HTTP {status} {reason or ''}."
    products, _ = extract_products(payload)
    if not products:
        return "products_by_ids: brak danych o produktach."
    lines = [summarize_product_entry(product) for product in products[:8]]
    return "products_by_ids:\n" + "\n".join(f"{idx + 1}. {line}" for idx, line in enumerate(lines))


def extract_cart_items(payload: object) -> list[dict]:
    normalized = parse_json_value(payload)
    if isinstance(normalized, dict):
        for key in ("products", "items"):
            value = normalized.get(key)
            if isinstance(value, list):
                return [item for item in value if isinstance(item, dict)]
        if isinstance(normalized.get("api_response"), dict):
            return extract_cart_items(normalized["api_response"])
    return []


def summarize_cart_entry(entry: dict) -> str:
    product = entry.get("product") if isinstance(entry.get("product"), dict) else entry
    name = localized_text(product.get("name") or product.get("displayName") or entry.get("name") or entry.get("displayName"))
    pid = compact_text(entry.get("productId") or product.get("productId") or entry.get("id"))
    qty = compact_text(entry.get("quantity") or 1)
    price = money_text(entry.get("price") or product.get("price"))
    total = money_text(entry.get("total"))
    parts = [name or pid or "pozycja"]
    if pid:
        parts.append(f"id={pid}")
    parts.append(f"ilość={qty}")
    if price:
        parts.append(f"cena/szt={price}")
    if total:
        parts.append(f"razem={total}")
    return " | ".join(parts)


def summarize_cart_output(raw: object, action: str) -> str:
    payload = parse_json_value(raw)
    status, reason = extract_http_error(payload)
    if status is not None:
        return f"{action}: błąd HTTP {status} {reason or ''}."
    items = extract_cart_items(payload)
    if not items:
        return f"{action}: koszyk jest pusty albo odpowiedź nie zawiera pozycji."
    lines = [summarize_cart_entry(item) for item in items[:8]]
    return f"{action}: koszyk ma {len(items)} pozycji.\n" + "\n".join(
        f"{idx + 1}. {line}" for idx, line in enumerate(lines)
    )


def summarize_reservation_slots_output(raw: object) -> str:
    payload = parse_json_value(raw)
    status, reason = extract_http_error(payload)
    if status is not None:
        return f"reservation_slots: błąd HTTP {status} {reason or ''}."
    if not isinstance(payload, dict):
        return "reservation_slots: brak czytelnych danych o slotach."

    days = payload.get("days")
    if not isinstance(days, list) or not days:
        return "reservation_slots: brak dostępnych slotów. Powiedz to jasno i zapytaj użytkownika o inny dzień lub sklep."

    lines: list[str] = []
    for day in days[:5]:
        if not isinstance(day, dict):
            continue
        date = compact_text(day.get("date")) or "nieznana data"
        slots = day.get("slots") if isinstance(day.get("slots"), list) else []
        slot_labels = []
        for slot in slots[:4]:
            if not isinstance(slot, dict):
                continue
            start = compact_text(slot.get("startsAt"))
            end = compact_text(slot.get("endsAt"))
            if "T" in start:
                start = start.split("T", 1)[1][:5]
            if "T" in end:
                end = end.split("T", 1)[1][:5]
            label = f"{start}-{end}".strip("-")
            if label:
                slot_labels.append(label)
        if slot_labels:
            lines.append(f"{date}: {', '.join(slot_labels)}")
        else:
            lines.append(f"{date}: brak slotów")

    return "reservation_slots:\n" + "\n".join(lines)


def summarize_session_output(raw: object, action: str) -> str:
    payload = parse_json_value(raw)
    if not isinstance(payload, dict):
        return f"{action}: brak czytelnej odpowiedzi."
    parts = [f"{action}:"]
    for key in ("saved", "token_saved", "refresh_token_saved", "cookie_saved", "user_id", "base_url"):
        if key in payload:
            parts.append(f"{key}={compact_text(payload.get(key))}")
    return " ".join(parts)


def build_tool_output_filters() -> dict[str, callable]:
    return {
        "products_search": summarize_products_search_output,
        "products_by_ids": summarize_products_by_ids_output,
        "cart_show": lambda raw: summarize_cart_output(raw, "cart_show"),
        "cart_add": lambda raw: summarize_cart_output(raw, "cart_add"),
        "cart_remove": lambda raw: summarize_cart_output(raw, "cart_remove"),
        "reservation_slots": summarize_reservation_slots_output,
        "session_login": lambda raw: summarize_session_output(raw, "session_login"),
        "session_refresh_token": lambda raw: summarize_session_output(raw, "session_refresh_token"),
    }


def build_registered_tool_output_filters(tools_schema: object) -> dict[str, callable]:
    filters = build_tool_output_filters()
    known_names: set[str] = set()

    if hasattr(tools_schema, "standard_tools"):
        for tool in list(getattr(tools_schema, "standard_tools", [])):
            name = getattr(tool, "name", "")
            if isinstance(name, str) and name.strip():
                known_names.add(name.strip())
        custom_tools = getattr(tools_schema, "custom_tools", {}) or {}
        for entries in custom_tools.values():
            for tool in entries:
                name = getattr(tool, "name", "")
                if isinstance(name, str) and name.strip():
                    known_names.add(name.strip())
    elif isinstance(tools_schema, (list, tuple)):
        for tool in tools_schema:
            if isinstance(tool, dict):
                name = tool.get("name", "")
            else:
                name = getattr(tool, "name", "")
            if isinstance(name, str) and name.strip():
                known_names.add(name.strip())

    return {name: fn for name, fn in filters.items() if name in known_names}


@dataclass
class ConsoleState:
    debug: bool = False
    running: bool = True
    show_transcripts: bool = True
    user_speaking: bool = False
    agent_speaking: bool = False
    _last_user_text: float = 0
    _last_agent_speech: float = 0
    _thinking_until: float = 0
    _last_status: str = ""

    def set_user_heard(self) -> None:
        self._last_user_text = time.monotonic()

    def set_user_speaking(self, speaking: bool) -> None:
        self.user_speaking = speaking
        if speaking:
            self._last_user_text = time.monotonic()

    def set_agent_speaking(self, speaking: bool = True) -> None:
        self.agent_speaking = speaking
        if speaking:
            self._last_agent_speech = time.monotonic()

    def set_thinking(self, ttl: float = 1.6) -> None:
        until = time.monotonic() + ttl
        if until > self._thinking_until:
            self._thinking_until = until

    def current_status(self) -> str:
        now = time.monotonic()
        if self.agent_speaking or now - self._last_agent_speech < 0.6:
            return "speak"
        if now < self._thinking_until:
            return "think"
        if self.user_speaking or now - self._last_user_text < 0.8:
            return "listen-user"
        return "listen"


STATUS_LABELS = {
    "listen": "Agent słucha (bez dźwięku)",
    "listen-user": "Słyszę Cię",
    "think": "Agent myśli (narzędzia MCP)",
    "speak": "Agent mówi",
}

STATUS_ANIM = {
    "listen": ["◌", "◉", "◌", "◌"],
    "listen-user": ["▐▌", "▋ ", "▌ ", " ▋"],
    "think": ["◴", "◷", "◶", "◵"],
    "speak": ["─", "━", "━", "─"],
}


class ConsoleObserver(BaseObserver):
    def __init__(self, *, state: ConsoleState, llm: OpenAIRealtimeLLMService, show_logs: bool = False):
        super().__init__()
        self._state = state
        self._llm = llm
        self._show_logs = show_logs
        self._last_user = None
        self._last_agent = None
        self._status_len = 0
        self._recent_log_keys: dict[str, float] = {}
        self._logged_tool_calls_in_progress: set[str] = set()
        self._logged_tool_calls_result: set[str] = set()

    def _should_log(self, key: str, ttl: float = 0.75) -> bool:
        now = time.monotonic()
        last = self._recent_log_keys.get(key, 0.0)
        if now - last < ttl:
            return False
        self._recent_log_keys[key] = now
        return True

    def _normalize_result(self, result: object) -> object:
        if isinstance(result, str):
            stripped = result.strip()
            if stripped.startswith("{") or stripped.startswith("["):
                with contextlib.suppress(Exception):
                    return json.loads(stripped)
        return result

    def _extract_http_status(self, result: object) -> tuple[int | None, str | None]:
        normalized = self._normalize_result(result)
        if isinstance(normalized, dict):
            status = normalized.get("status")
            reason = normalized.get("reason")
            if isinstance(status, int):
                return status, str(reason) if reason else None
        return None, None

    def _print_log(self, message: str) -> None:
        if not self._show_logs:
            return
        if self._status_len:
            sys.stderr.write("\r" + (" " * self._status_len) + "\r")
        ts = time.strftime("%H:%M:%S")
        print(f"[{ts}] [voice] {message}", file=sys.stderr, flush=True)

    def _preview(self, value: object, *, max_len: int = 220) -> str:
        try:
            text = json.dumps(value, ensure_ascii=False)
        except Exception:
            text = str(value)
        text = " ".join(text.split())
        if len(text) > max_len:
            return text[: max_len - 1] + "…"
        return text

    async def on_push_frame(self, data: FramePushed):
        frame = data.frame

        if isinstance(frame, UserStartedSpeakingFrame):
            self._state.set_user_speaking(True)
            return

        if isinstance(frame, UserStoppedSpeakingFrame):
            self._state.set_user_speaking(False)
            return

        if isinstance(frame, BotStartedSpeakingFrame):
            if not self._state.agent_speaking:
                self._state.set_agent_speaking(True)
                self._llm.set_audio_input_paused(True)
                if self._should_log("agent_started", ttl=0.5):
                    self._print_log("agent zaczął mówić; wejście audio użytkownika tymczasowo wstrzymane")
            return

        if isinstance(frame, BotStoppedSpeakingFrame):
            if self._state.agent_speaking:
                self._state.set_agent_speaking(False)
                self._llm.set_audio_input_paused(False)
                if self._should_log("agent_stopped", ttl=0.5):
                    self._print_log("agent skończył mówić; wejście audio użytkownika wznowione")
            return

        if isinstance(frame, TranscriptionFrame):
            text = (frame.text or "").strip()
            if text:
                self._state.set_user_heard()
            if self._state.show_transcripts and frame.finalized and text and text != self._last_user:
                self._last_user = text
                print(f"\nTy: {text}", flush=True)
            return

        if isinstance(frame, TTSTextFrame):
            text = (frame.text or "").strip()
            aggregated_by = getattr(frame, "aggregated_by", "")
            aggregated_by = getattr(aggregated_by, "value", str(aggregated_by)).strip().lower()
            if (
                text
                and self._state.show_transcripts
                and aggregated_by == "sentence"
                and text != self._last_agent
            ):
                self._last_agent = text
                print(f"\nAgent: {text}", flush=True)
            return

        if isinstance(frame, FunctionCallsStartedFrame):
            names = ", ".join(call.function_name for call in frame.function_calls)
            self._state.set_thinking(ttl=3.0)
            if self._should_log(f"tools_started:{names}", ttl=1.0):
                self._print_log(f"LLM uruchamia narzędzia: {names}")
            return

        if isinstance(frame, FunctionCallInProgressFrame):
            self._state.set_thinking(ttl=3.0)
            key = frame.tool_call_id or f"{frame.function_name}:{self._preview(frame.arguments)}"
            if key not in self._logged_tool_calls_in_progress:
                self._logged_tool_calls_in_progress.add(key)
                self._print_log(
                    f"MCP call -> {frame.function_name} args={self._preview(frame.arguments)}"
                )
            if self._state.debug:
                print(
                    f"\n[tool] {frame.function_name}({frame.arguments})",
                    file=sys.stderr,
                    flush=True,
                )
            return

        if isinstance(frame, FunctionCallResultFrame):
            self._state.set_thinking(ttl=1.0)
            key = frame.tool_call_id or f"{frame.function_name}:{self._preview(frame.result)}"
            if key not in self._logged_tool_calls_result:
                self._logged_tool_calls_result.add(key)
                self._print_log(
                    f"MCP result <- {frame.function_name} result={self._preview(frame.result)}"
                )
                status, reason = self._extract_http_status(frame.result)
                if status == 401:
                    self._print_log(
                        f"MCP auth problem: {frame.function_name} zwrócił 401 {reason or 'Unauthorized'}; trzeba zalogować sesję sklepu"
                    )
            return

    async def status_loop(self) -> None:
        i = 0
        while self._state.running:
            status = self._state.current_status()
            frames = STATUS_ANIM.get(status, ["."])
            status_text = f"{frames[i % len(frames)]} {STATUS_LABELS.get(status, status)}"
            clear = " " * max(0, self._status_len - len(status_text))
            self._status_len = max(self._status_len, len(status_text))
            sys.stderr.write(f"\r{status_text}{clear}")
            sys.stderr.flush()

            if status != self._state._last_status:
                self._state._last_status = status
                i = 0
            else:
                i += 1
            await asyncio.sleep(0.25)

        sys.stderr.write("\r" + (" " * self._status_len) + "\r")
        sys.stderr.flush()


async def watch_for_spacebar(
    task: PipelineTask,
    observer: ConsoleObserver,
    llm: OpenAIRealtimeLLMService,
    state: ConsoleState,
) -> None:
    if termios is None or tty is None:
        return
    if not sys.stdin.isatty():
        return

    fd = sys.stdin.fileno()
    try:
        original = termios.tcgetattr(fd)
    except termios.error:
        return

    tty.setcbreak(fd)
    last_interrupt_at = 0.0
    try:
        while True:
            await asyncio.sleep(0.05)
            try:
                readable, _, _ = select.select([fd], [], [], 0)
            except (OSError, ValueError):
                return
            if not readable:
                continue

            try:
                char = os.read(fd, 1).decode("utf-8", errors="ignore")
            except OSError:
                return
            if char != " ":
                continue

            now = time.monotonic()
            if now - last_interrupt_at < 0.35:
                continue

            if state.agent_speaking:
                last_interrupt_at = now
                state.set_agent_speaking(False)
                llm.set_audio_input_paused(False)
                observer._print_log("spacebar -> przerywam agenta, możesz mówić")
                await task.queue_frame(InterruptionFrame())
    finally:
        with contextlib.suppress(termios.error, OSError):
            termios.tcsetattr(fd, termios.TCSADRAIN, original)


async def run_agent(args: argparse.Namespace) -> bool:
    api_key = os.getenv("OPENAI_API_KEY", "").strip()
    if not api_key:
        raise SystemExit("OPENAI_API_KEY is required. Export it in your shell before running `martmart voice`.")

    logger.remove()
    logger.add(sys.stderr, level="DEBUG" if args.debug else "WARNING")

    session_properties = events.SessionProperties(
        output_modalities=["audio"],
        audio=events.AudioConfiguration(
            input=events.AudioInput(
                transcription=events.InputAudioTranscription(
                    model=args.transcription_model,
                    language=args.language,
                    prompt=(
                        "Polish grocery shopping conversation, recipe ideas, food brands, grocery providers, "
                        "product names, ingredients, promotions, substitutes, complementary items."
                    ),
                ),
                turn_detection=events.TurnDetection(),
            ),
            output=events.AudioOutput(
                voice=args.voice,
                speed=args.voice_speed,
            ),
        ),
        tool_choice="auto",
    )

    llm = OpenAIRealtimeLLMService(
        api_key=api_key,
        settings=OpenAIRealtimeLLMService.Settings(
            model=args.model,
            system_instruction=SYSTEM_PROMPT,
            filter_incomplete_user_turns=True,
            session_properties=session_properties,
        ),
    )

    transport_params = LocalAudioTransportParams(
        audio_in_enabled=True,
        audio_out_enabled=True,
    )
    if args.input_device >= 0:
        transport_params.input_device_index = args.input_device
    if args.output_device >= 0:
        transport_params.output_device_index = args.output_device
    transport = LocalAudioTransport(transport_params)

    mcp_server = StdioServerParameters(
        command=args.martmart_binary,
        args=args.mcp_args,
    )

    console_state = ConsoleState(debug=args.debug, show_transcripts=True)
    observer = ConsoleObserver(state=console_state, llm=llm, show_logs=args.show_logs or args.debug)

    status_task = asyncio.create_task(observer.status_loop())

    try:
        if args.show_logs or args.debug:
            observer._print_log(
                f"start MCP stdio: {args.martmart_binary} {' '.join(args.mcp_args)}"
            )

        async with MCPClient(
            server_params=mcp_server,
        ) as mcp:
            tools_schema = await mcp.get_tools_schema()
            mcp.tools_output_filters = build_registered_tool_output_filters(tools_schema)
            await mcp.register_tools_schema(tools_schema, llm)

            if args.show_logs or args.debug:
                discovered_tool_names: list[str] = []
                if hasattr(tools_schema, "standard_tools"):
                    standard_tools = list(getattr(tools_schema, "standard_tools", []))
                    custom_tools = getattr(tools_schema, "custom_tools", {}) or {}
                    discovered_tool_names.extend(
                        getattr(tool, "name", "?") for tool in standard_tools
                    )
                    for entries in custom_tools.values():
                        discovered_tool_names.extend(getattr(tool, "name", "?") for tool in entries)
                    total_tools = len(discovered_tool_names)
                else:
                    standard_tools = list(tools_schema) if isinstance(tools_schema, (list, tuple)) else []
                    discovered_tool_names.extend(
                        tool.get("name", "?") if isinstance(tool, dict) else getattr(tool, "name", "?")
                        for tool in standard_tools
                    )
                    total_tools = len(discovered_tool_names)

                tool_names = ", ".join(discovered_tool_names)
                observer._print_log(f"zarejestrowano MCP tools ({total_tools}): {tool_names}")
                provider_tool_hint = [
                    name
                    for name in ["providers_list", "providers_available", "provider_list", "session_list"]
                    if name in discovered_tool_names
                ]
                if provider_tool_hint:
                    observer._print_log(
                        "narzędzia do wykrywania providerów dostępne: " + ", ".join(provider_tool_hint)
                    )

            kickoff_context = LLMContext(
                messages=[
                    {
                        "role": "user",
                        "content": args.kickoff_prompt,
                    }
                ],
                tools=tools_schema,
                tool_choice="auto",
            )
            assistant_context = LLMAssistantAggregator(kickoff_context)

            pipeline = Pipeline([
                transport.input(),
                llm,
                transport.output(),
                assistant_context,
            ])
            task = PipelineTask(
                pipeline,
                params=PipelineParams(
                    audio_in_sample_rate=16000,
                    audio_out_sample_rate=24000,
                ),
                observers=[observer],
            )

            await task.queue_frame(LLMContextFrame(kickoff_context))

            print("MartMart voice agent started with MCP tool discovery.", flush=True)
            print("Mów normalnie po polsku. Agent może sprawdzić dostępnych providerów i wybrać sklep per request. Spacja przerywa agenta i oddaje głos Tobie. Ctrl+C kończy sesję.", flush=True)
            print("", flush=True)

            runner = PipelineRunner(handle_sigint=True)
            spacebar_task = asyncio.create_task(watch_for_spacebar(task, observer, llm, console_state))
            try:
                await runner.run(task)
            finally:
                if not spacebar_task.done():
                    spacebar_task.cancel()
                    with contextlib.suppress(asyncio.CancelledError):
                        await spacebar_task

            return runner._sig_task is not None
    except asyncio.CancelledError:
        raise
    finally:
        console_state.running = False
        await status_task

def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="MartMart Pipecat voice shopping assistant with MCP provider discovery")
    parser.add_argument("--martmart-binary", required=True)
    parser.add_argument("--model", default="gpt-realtime")
    parser.add_argument("--voice", default="alloy")
    parser.add_argument("--voice-speed", type=float, default=1.0)
    parser.add_argument("--language", default="pl")
    parser.add_argument("--transcription-model", default="gpt-4o-transcribe")
    parser.add_argument("--kickoff-prompt", default=KICKOFF_PROMPT)
    parser.add_argument("--input-device", type=int, default=-1)
    parser.add_argument("--output-device", type=int, default=-1)
    parser.add_argument("--debug", action="store_true")
    parser.add_argument("--show-logs", action="store_true")
    parser.add_argument("mcp_args", nargs="*")

    parsed, unknown = parser.parse_known_args()

    mcp_args = list(unknown)
    if not mcp_args and parsed.mcp_args:
        mcp_args = parsed.mcp_args

    if mcp_args and mcp_args[0] == "--":
        mcp_args = mcp_args[1:]

    parsed.mcp_args = mcp_args
    return parsed


def format_duration(seconds: float) -> str:
    minutes, secs = divmod(int(seconds), 60)
    if minutes:
        return f"{minutes} min {secs:02d} s"
    return f"{secs} s"


def print_session_footer(reason: str, start_time: float | None = None) -> None:
    elapsed = None
    if start_time is not None:
        elapsed = format_duration(time.monotonic() - start_time)

    if elapsed:
        print(f"\nZakończono sesję głosową ({reason}). Czas: {elapsed}.", file=sys.stderr)
    else:
        print(f"\nZakończono sesję głosową ({reason}).", file=sys.stderr)


def validate_args(args: argparse.Namespace) -> None:
    if not args.mcp_args:
        raise SystemExit("Missing MCP command arguments. Expected something like: mcp")

def main() -> None:
    args = parse_args()
    validate_args(args)
    start_time = time.monotonic()

    try:
        interrupted = asyncio.run(run_agent(args))
        if interrupted:
            print_session_footer("przerwane przez użytkownika", start_time)
        else:
            print_session_footer("zakończono poprawnie", start_time)
    except (KeyboardInterrupt, asyncio.CancelledError):
        print_session_footer("przerwane przez użytkownika", start_time)
    except Exception as exc:
        print_session_footer("zakończono z błędem", start_time)
        raise SystemExit(str(exc)) from exc

if __name__ == "__main__":
    main()
