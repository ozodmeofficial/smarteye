// Remote view & control window (AnyDesk-style). Shows a device's live screen,
// and in control mode forwards mouse/keyboard input to the agent.
import { el, $ } from "./util.js";
import { t } from "./i18n.js";
import { icon } from "./icons.js";
import { send, on } from "./bus.js";
import { toast } from "./dialogs.js";

let active = null; // { id, monitor, controlling, img, off }

export function openControl(device) {
  closeControl();
  const img = el("img", { alt: "" });
  const stage = el("div.rc-stage", {}, [img, el("div.rc-hint", { id: "rcHint", text: t("view_hint") })]);

  const monitors = device.monitors || [];
  const monSel = el("select.select", { style: "height:34px;max-width:150px;background:#241d18;border-color:#2f2720;color:#d8cebf" },
    monitors.map((m, i) => el("option", { value: i, text: t("monitor") + " " + (i + 1) + " (" + m.width + "×" + m.height + ")" })));
  if (monitors.length < 2) monSel.style.display = "none";

  const seg = el("div.seg", {}, [
    el("button.active", { "data-mode": "view", text: t("view_only") }),
    el("button", { "data-mode": "control", text: t("full_control") }),
  ]);

  const bar = el("div.rc-bar", {}, [
    el("div.title", { html: `${device.hostname}<span class="sub">${device.username || ""}</span>` }),
    el("div.spacer"),
    seg,
    monSel,
    el("button.rcbtn", { html: icon("keyboard") + "<span>" + t("send_cad") + "</span>", title: "Ctrl+Alt+Del",
      onclick: sendCAD }),
    el("button.rcbtn", { html: icon("camera"), title: t("screenshot"), onclick: () => screenshot(device) }),
    el("button.rcbtn", { html: icon("expand"), title: t("fullscreen"), onclick: toggleFS }),
    el("button.rcbtn.close", { html: icon("x") + "<span>" + t("cancel") + "</span>", onclick: closeControl }),
  ]);

  const rc = el("div.rc", {}, [bar, stage]);
  document.body.appendChild(rc);

  active = { id: device.id, monitor: 0, controlling: false, img, stage, rc };

  // Receive frames for this device.
  const off = on("frame", (f) => {
    if (!active || f.id !== active.id) return;
    img.src = "data:image/jpeg;base64," + f.data;
  });
  active.off = off;

  send("view_start", { device: device.id, monitor: 0, fps: 24, quality: 72 });

  // Mode switching.
  seg.querySelectorAll("button").forEach((b) =>
    b.addEventListener("click", () => {
      seg.querySelectorAll("button").forEach((x) => x.classList.remove("active"));
      b.classList.add("active");
      setControlling(b.dataset.mode === "control");
    }));

  monSel.addEventListener("change", () => {
    active.monitor = +monSel.value;
    send("select_monitor", { device: active.id, monitor: active.monitor });
  });

  wireInput(img);
}

function setControlling(on_) {
  if (!active) return;
  active.controlling = on_;
  active.stage.classList.toggle("controlling", on_);
  $("#rcHint").textContent = on_ ? t("control_hint") : t("view_hint");
  send("control", { device: active.id, control: on_, freeze: on_ });
}

// Map a mouse event on the <img> to normalized 0..1 coords over the real image,
// accounting for object-fit: contain letterboxing.
function normCoords(img, e) {
  const r = img.getBoundingClientRect();
  const nW = img.naturalWidth || r.width, nH = img.naturalHeight || r.height;
  const scale = Math.min(r.width / nW, r.height / nH);
  const dispW = nW * scale, dispH = nH * scale;
  const offX = r.left + (r.width - dispW) / 2;
  const offY = r.top + (r.height - dispH) / 2;
  let x = (e.clientX - offX) / dispW;
  let y = (e.clientY - offY) / dispH;
  x = Math.max(0, Math.min(1, x));
  y = Math.max(0, Math.min(1, y));
  return { x, y };
}

const BUTTONS = { 0: "left", 1: "middle", 2: "right" };

function wireInput(img) {
  let lastMove = 0;
  img.addEventListener("mousemove", (e) => {
    if (!active?.controlling) return;
    const now = performance.now();
    if (now - lastMove < 16) return; // ~60Hz cap
    lastMove = now;
    const { x, y } = normCoords(img, e);
    send("input", { device: active.id, event: { kind: "mouse_move", x, y } });
  });
  img.addEventListener("mousedown", (e) => {
    if (!active?.controlling) return;
    e.preventDefault();
    const { x, y } = normCoords(img, e);
    send("input", { device: active.id, event: { kind: "mouse_down", x, y, button: BUTTONS[e.button] || "left" } });
  });
  img.addEventListener("mouseup", (e) => {
    if (!active?.controlling) return;
    e.preventDefault();
    const { x, y } = normCoords(img, e);
    send("input", { device: active.id, event: { kind: "mouse_up", x, y, button: BUTTONS[e.button] || "left" } });
  });
  img.addEventListener("contextmenu", (e) => e.preventDefault());
  img.addEventListener("wheel", (e) => {
    if (!active?.controlling) return;
    e.preventDefault();
    const { x, y } = normCoords(img, e);
    send("input", { device: active.id, event: { kind: "wheel", x, y, delta: -Math.sign(e.deltaY) * 120 } });
  }, { passive: false });

  // Keyboard, captured at document level while controlling.
  document.addEventListener("keydown", onKey, true);
  document.addEventListener("keyup", onKey, true);
}

function onKey(e) {
  if (!active?.controlling) return;
  // Let Escape close the window instead of being forwarded.
  if (e.key === "Escape") { closeControl(); return; }
  e.preventDefault();
  e.stopPropagation();
  const kind = e.type === "keydown" ? "key_down" : "key_up";
  send("input", { device: active.id, event: { kind, key_code: e.keyCode || e.which, key: e.key } });
}

function sendCAD() {
  if (!active) return;
  // Ctrl(17)+Alt(18)+Delete(46) down then up.
  const seq = [
    ["key_down", 17], ["key_down", 18], ["key_down", 46],
    ["key_up", 46], ["key_up", 18], ["key_up", 17],
  ];
  seq.forEach(([kind, code]) => send("input", { device: active.id, event: { kind, key_code: code } }));
  toast("Ctrl+Alt+Del", t("toast_sent"), "ok");
}

function screenshot(device) {
  if (!active?.img?.src) return;
  const a = document.createElement("a");
  a.href = active.img.src;
  a.download = `${device.hostname}-${Date.now()}.jpg`;
  a.click();
  toast(t("screenshot"), t("toast_sent"), "ok");
}

function toggleFS() {
  if (!document.fullscreenElement) active?.rc.requestFullscreen?.();
  else document.exitFullscreen?.();
}

export function closeControl() {
  if (!active) return;
  send("view_stop", { device: active.id });
  if (active.controlling) send("control", { device: active.id, control: false, freeze: false });
  active.off?.();
  document.removeEventListener("keydown", onKey, true);
  document.removeEventListener("keyup", onKey, true);
  active.rc.remove();
  if (document.fullscreenElement) document.exitFullscreen?.();
  active = null;
}
