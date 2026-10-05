// SmartEYE dashboard entry point: state, WebSocket, and rendering.
import { $, $$, el, esc } from "./util.js";
import { t, setLang, getLang } from "./i18n.js";
import { icon } from "./icons.js";
import { setSender, emit } from "./bus.js";
import {
  toast, lockDialog, messageDialog, launchDialog, roomDialog, assignDialog, powerMenu, roomMenu, closeMenus,
} from "./dialogs.js";
import { openControl } from "./control.js";

const state = {
  devices: {},      // id -> device
  rooms: [],        // [{id,name}]
  thumbs: {},       // id -> {data,seq}
  selected: new Set(),
  room: "all",      // current filter: "all" | "found" | roomId
  search: "",
  info: {},
};

// ---------------- WebSocket ----------------
let ws, reconnectTimer;
function connect() {
  ws = new WebSocket((location.protocol === "https:" ? "wss://" : "ws://") + location.host + "/ws");
  ws.onopen = () => { showDisconnected(false); };
  ws.onclose = () => { showDisconnected(true); scheduleReconnect(); };
  ws.onerror = () => ws.close();
  ws.onmessage = (ev) => {
    const msg = JSON.parse(ev.data);
    handle(msg);
  };
}
function scheduleReconnect() {
  clearTimeout(reconnectTimer);
  reconnectTimer = setTimeout(connect, 1500);
}
function wsSend(type, payload) {
  if (ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type, payload }));
}
setSender(wsSend);

function handle(msg) {
  switch (msg.type) {
    case "snapshot": {
      state.rooms = msg.payload.rooms || [];
      const map = {};
      (msg.payload.devices || []).forEach((d) => { map[d.id] = d; });
      state.devices = map;
      // Drop selections for devices that vanished.
      state.selected.forEach((id) => { if (!map[id]) state.selected.delete(id); });
      refresh();
      break;
    }
    case "thumb": {
      const p = msg.payload;
      state.thumbs[p.id] = { data: p.data, seq: p.seq };
      updateThumb(p.id);
      break;
    }
    case "frame":
      emit("frame", msg.payload);
      break;
    case "clipboard":
      emit("clipboard", msg.payload);
      break;
  }
}

function showDisconnected(on) {
  const d = $("#disc");
  d.classList.toggle("show", on);
  $("#discText").textContent = t("disconnected");
}

// ---------------- Derived ----------------
function visibleDevices() {
  let list = Object.values(state.devices);
  if (state.room === "found") list = list.filter((d) => !d.room);
  else if (state.room !== "all") list = list.filter((d) => d.room === state.room);
  if (state.search) {
    const q = state.search.toLowerCase();
    list = list.filter((d) =>
      (d.hostname || "").toLowerCase().includes(q) ||
      (d.username || "").toLowerCase().includes(q) ||
      (d.foreground_app || "").toLowerCase().includes(q));
  }
  // Online first, then by hostname.
  list.sort((a, b) => (b.online - a.online) || (a.hostname || "").localeCompare(b.hostname || ""));
  return list;
}
function roomName(id) { const r = state.rooms.find((x) => x.id === id); return r ? r.name : ""; }
function onlineCount() { return Object.values(state.devices).filter((d) => d.online).length; }

// ---------------- Render ----------------
function render() {
  renderTopbar();
  renderSidebar();
  renderMain();
}

// refresh updates the UI from a new snapshot with minimal DOM churn: the grid
// is only rebuilt when the set/order of visible tiles changes. Otherwise each
// tile is patched in place, so live status updates every couple of seconds
// never cause flicker or interrupt hovering, menus or selection.
let lastGridKey = null;
function refresh() {
  renderTopbar();
  renderSidebar();
  if (document.querySelector(".overlay")) { /* a dialog is open: still patch tiles below */ }

  const list = visibleDevices();
  const key = state.room + "|" + state.search + "|" + list.map((d) => d.id).join(",");
  const grid = $("#main .grid");
  if (!grid || key !== lastGridKey) {
    renderMain();
    lastGridKey = key;
    return;
  }
  // Same tiles: patch each in place.
  list.forEach((d) => patchTile(d));
  // Update the "N computers" count.
  const cnt = $("#main .page-head .muted");
  if (cnt) cnt.textContent = list.length + " " + t("computers");
}

function patchTile(d) {
  const tile = document.getElementById("tile-" + cssId(d.id));
  if (!tile) return;
  tile.classList.toggle("offline", !d.online);
  tile.classList.toggle("selected", state.selected.has(d.id));
  const dot = tile.querySelector(".status-dot");
  if (dot) dot.className = "status-dot " + (!d.online ? "" : d.locked ? "locked" : "online");
  const user = tile.querySelector(".user");
  if (user) user.textContent = d.username || "";
  const fg = tile.querySelector(".fg");
  if (fg) {
    fg.innerHTML = "";
    if (d.online && d.foreground_app) {
      fg.appendChild(el("span.app", { text: d.foreground_app }));
      fg.appendChild(el("span.title", { text: d.foreground_title || "" }));
    } else {
      fg.appendChild(el("span.title", { text: d.os || "" }));
    }
  }
  // Badges (lock / latency).
  const badges = tile.querySelector(".badges");
  if (badges) {
    badges.innerHTML = "";
    if (d.locked) badges.appendChild(el("span.pill.locked", { html: icon("lock") + t("locked") }));
    if (d.online && d.latency_ms) badges.appendChild(el("span.pill.lat", { text: d.latency_ms + "ms" }));
  }
}

function renderTopbar() {
  $("#serverName").textContent = state.info.server_name || "SmartEYE";
  $("#netCode").textContent = state.info.net_code || "— — —";
  $("#onlineCount").textContent = onlineCount();
  $("#totalCount").textContent = Object.keys(state.devices).length;
}

function renderSidebar() {
  const nav = $("#sidebar");
  nav.innerHTML = "";
  const all = Object.values(state.devices);
  const foundCount = all.filter((d) => !d.room).length;

  const itemRow = (id, ic, name, count, active, menu) => {
    const r = el("div.room" + (active ? ".active" : ""), {
      onclick: () => { state.room = id; state.selected.clear(); render(); },
    }, [
      el("span.ic", { html: icon(ic) }),
      el("span.name", { text: name }),
      count != null ? el("span.count", { text: String(count) }) : null,
    ]);
    if (menu) {
      r.appendChild(el("button.iconbtn.mini", {
        style: "width:26px;height:26px",
        html: icon("dots"),
        onclick: (e) => { e.stopPropagation(); const b = e.currentTarget.getBoundingClientRect(); menu(b.left, b.bottom + 4); },
      }));
    }
    return r;
  };

  nav.appendChild(itemRow("all", "grid", t("all_computers"), all.length, state.room === "all"));
  if (foundCount > 0)
    nav.appendChild(itemRow("found", "wifi", t("found"), foundCount, state.room === "found"));

  nav.appendChild(el("div.section-label", { text: t("rooms") }));
  state.rooms.forEach((rm) => {
    const count = all.filter((d) => d.room === rm.id).length;
    nav.appendChild(itemRow(rm.id, "folder", rm.name, count, state.room === rm.id,
      (x, y) => roomMenu(x, y, rm)));
  });
  nav.appendChild(el("div.addroom", { html: icon("plus") + "<span>" + t("new_room") + "</span>", onclick: () => roomDialog(null) }));
}

function renderMain() {
  const main = $("#main");
  main.innerHTML = "";
  const list = visibleDevices();

  // Action bar when selection is active.
  if (state.selected.size > 0) main.appendChild(renderActionBar());

  // Page head.
  const headTitle = state.room === "all" ? t("all_computers")
    : state.room === "found" ? t("found") : roomName(state.room);
  main.appendChild(el("div.page-head", {}, [
    el("h2", { text: headTitle }),
    el("span.muted", { text: list.length + " " + t("computers") }),
  ]));

  if (list.length === 0) {
    main.appendChild(el("div.empty", {}, [
      el("div.ic", { html: icon("monitor") }),
      el("h3", { text: t("empty_title") }),
      el("div", { text: t("empty_sub") }),
    ]));
    return;
  }

  const grid = el("div.grid");
  list.forEach((d) => grid.appendChild(renderTile(d)));
  main.appendChild(grid);
  // Keep the reconciliation key in sync so the next snapshot patches in place.
  lastGridKey = state.room + "|" + state.search + "|" + list.map((d) => d.id).join(",");
}

function renderActionBar() {
  const targets = () => Array.from(state.selected);
  const n = state.selected.size;
  const bar = el("div.actionbar");
  bar.appendChild(el("span.sel-count", { text: n + " " + t("selected") }));
  bar.appendChild(el("button.btn.ghost", { text: t("clear"), onclick: () => {
    state.selected.clear();
    document.querySelectorAll(".tile.selected").forEach((x) => x.classList.remove("selected"));
    updateActionBar();
  } }));
  bar.appendChild(el("div.sep"));

  const act = (ic, label, cls, fn) => el("button.btn" + (cls ? "." + cls : ""),
    { html: icon(ic) + "<span>" + label + "</span>", onclick: fn });

  bar.appendChild(act("lock", t("lock"), "", () => lockDialog(targets())));
  bar.appendChild(act("unlock", t("unlock"), "", () => { wsSend("unlock", { targets: targets() }); toast(t("unlock"), "", "ok"); }));
  bar.appendChild(act("message", t("message"), "", () => messageDialog(targets())));
  bar.appendChild(act("rocket", t("launch"), "", () => launchDialog(targets())));
  bar.appendChild(act("power", t("power"), "danger", (e) => { const b = e.currentTarget.getBoundingClientRect(); powerMenu(b.left, b.bottom + 4, targets()); }));
  bar.appendChild(el("div.sep"));
  bar.appendChild(act("folder", t("assign_room"), "", () => assignDialog(targets(), state.rooms)));
  return bar;
}

function renderTile(d) {
  const sel = state.selected.has(d.id);
  const statusCls = !d.online ? "" : d.locked ? "locked" : "online";
  const th = state.thumbs[d.id];

  const screen = el("div.screen", {}, [
    th && d.online
      ? el("img", { id: "thumb-" + cssId(d.id), src: "data:image/jpeg;base64," + th.data })
      : el("div.noimg", { html: icon("monitor") + "<span>" + (d.online ? t("no_preview") : t("offline")) + "</span>" }),
    el("div.check", { html: icon("check") }),
    el("div.badges", {}, [
      d.locked ? el("span.pill.locked", { html: icon("lock") + t("locked") }) : null,
      d.online && d.latency_ms ? el("span.pill.lat", { text: d.latency_ms + "ms" }) : null,
    ]),
  ]);

  const info = el("div.info", {}, [
    el("div.row1", {}, [
      el("span.status-dot " + statusCls, {}),
      el("span.host", { text: d.hostname || "—" }),
      el("span.user", { text: d.username || "" }),
    ]),
    el("div.fg", {}, d.online && d.foreground_app
      ? [el("span.app", { text: d.foreground_app }), el("span.title", { text: d.foreground_title || "" })]
      : [el("span.title", { text: d.os || "" })]),
  ]);

  const tile = el("div.tile" + (sel ? ".selected" : "") + (d.online ? "" : ".offline"),
    { id: "tile-" + cssId(d.id) }, [screen, info]);

  // Click = toggle selection; double-click = open control.
  let clickTimer;
  tile.addEventListener("click", (e) => {
    if (e.target.closest(".check")) { toggleSelect(d.id); return; }
    clearTimeout(clickTimer);
    clickTimer = setTimeout(() => toggleSelect(d.id), 180);
  });
  tile.addEventListener("dblclick", () => {
    clearTimeout(clickTimer);
    if (d.online) openControl(d);
    else toast(d.hostname, t("offline"));
  });
  return tile;
}

function toggleSelect(id) {
  if (state.selected.has(id)) state.selected.delete(id);
  else state.selected.add(id);
  // Patch only the affected tile and the action bar; never rebuild the grid, so
  // multi-select stays smooth and hover is never interrupted.
  const tile = document.getElementById("tile-" + cssId(id));
  if (tile) tile.classList.toggle("selected", state.selected.has(id));
  updateActionBar();
}

// updateActionBar inserts, updates or removes the selection toolbar in place.
function updateActionBar() {
  const main = $("#main");
  const existing = $(".actionbar", main);
  if (state.selected.size === 0) {
    if (existing) existing.remove();
    return;
  }
  const fresh = renderActionBar();
  if (existing) existing.replaceWith(fresh);
  else main.insertBefore(fresh, main.firstChild);
}

// Fast-path thumbnail update without a full re-render.
function updateThumb(id) {
  const data = "data:image/jpeg;base64," + state.thumbs[id].data;
  const img = document.getElementById("thumb-" + cssId(id));
  if (img) { img.src = data; return; }
  // First frame for this tile: swap the placeholder for an image in place,
  // without rebuilding the grid (keeps hover/selection intact).
  const tile = document.getElementById("tile-" + cssId(id));
  if (tile && state.devices[id]?.online) {
    const screen = tile.querySelector(".screen");
    const noimg = screen && screen.querySelector(".noimg");
    if (noimg) {
      noimg.replaceWith(el("img", { id: "thumb-" + cssId(id), src: data }));
    }
  }
}
function cssId(id) { return id.replace(/[^a-zA-Z0-9_-]/g, ""); }

// ---------------- Chrome (lang / theme / search) ----------------
function applyStaticIcons() {
  $("#logo").innerHTML = icon("eye");
  $("#searchIcon").innerHTML = icon("search");
  $("#search").placeholder = t("search");
  updateThemeBtn();
}
function updateThemeBtn() {
  const dark = document.documentElement.getAttribute("data-theme") === "dark";
  $("#themeBtn").innerHTML = icon(dark ? "sun" : "moon");
}
function setTheme(theme) {
  document.documentElement.setAttribute("data-theme", theme);
  localStorage.setItem("se_theme", theme);
  updateThemeBtn();
  savePrefs();
}
function setLanguage(l) {
  setLang(l);
  document.documentElement.lang = l;
  localStorage.setItem("se_lang", l);
  $$("#langSeg button").forEach((b) => b.classList.toggle("active", b.dataset.lang === l));
  applyStaticIcons();
  render();
  savePrefs();
}
function savePrefs() {
  fetch("/api/prefs", {
    method: "POST", headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ language: getLang(), theme: document.documentElement.getAttribute("data-theme") }),
  }).catch(() => {});
}

function wireChrome() {
  $("#themeBtn").addEventListener("click", () =>
    setTheme(document.documentElement.getAttribute("data-theme") === "dark" ? "light" : "dark"));
  $$("#langSeg button").forEach((b) =>
    b.addEventListener("click", () => setLanguage(b.dataset.lang)));
  $("#search").addEventListener("input", (e) => { state.search = e.target.value; renderMain(); });
  $("#codeChip").addEventListener("click", () => {
    navigator.clipboard?.writeText((state.info.net_code || "").replace(/\s/g, ""));
    toast(t("net_code"), "✓");
  });
  window.addEventListener("resize", closeMenus);
}

// ---------------- Boot ----------------
async function boot() {
  // Restore local prefs first for instant correct theme/lang.
  const savedTheme = localStorage.getItem("se_theme");
  const savedLang = localStorage.getItem("se_lang");

  try {
    const info = await (await fetch("/api/info")).json();
    state.info = info;
    if (!savedTheme && info.theme && info.theme !== "system") document.documentElement.setAttribute("data-theme", info.theme);
    if (!savedLang && info.language) setLang(info.language);
  } catch {}

  if (savedTheme) document.documentElement.setAttribute("data-theme", savedTheme);
  else if (!document.documentElement.getAttribute("data-theme")) {
    document.documentElement.setAttribute("data-theme",
      matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
  }
  const lang = savedLang || state.info.language || "uz";
  setLang(lang);
  document.documentElement.lang = lang;
  $$("#langSeg button").forEach((b) => b.classList.toggle("active", b.dataset.lang === lang));

  applyStaticIcons();
  wireChrome();
  renderTopbar();
  render();
  connect();
}

boot();
