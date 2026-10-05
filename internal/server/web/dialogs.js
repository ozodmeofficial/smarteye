// Modal dialogs, dropdown menus and toasts for the SmartEYE dashboard.
import { el, $ } from "./util.js";
import { t } from "./i18n.js";
import { icon } from "./icons.js";
import { send } from "./bus.js";

// --- Toasts -----------------------------------------------------------------
export function toast(title, msg, kind = "") {
  const box = $("#toasts");
  const node = el("div.toast" + (kind ? "." + kind : ""), {}, [
    el("div.t", { text: title }),
    msg ? el("div.m", { text: msg }) : null,
  ]);
  box.appendChild(node);
  setTimeout(() => {
    node.style.transition = "opacity .3s, transform .3s";
    node.style.opacity = "0";
    node.style.transform = "translateX(30px)";
    setTimeout(() => node.remove(), 300);
  }, 3200);
}

// --- Generic modal ----------------------------------------------------------
function modal({ title, sub, body, actions }) {
  const overlay = el("div.overlay");
  const dialog = el("div.dialog", {}, [
    el("header", {}, [el("h3", { text: title }), sub ? el("div.sub", { text: sub }) : null]),
    el("div.body", {}, body),
    el("footer", {}, actions),
  ]);
  overlay.appendChild(dialog);
  const close = () => overlay.remove();
  overlay.addEventListener("mousedown", (e) => { if (e.target === overlay) close(); });
  document.addEventListener("keydown", function esc(e) {
    if (e.key === "Escape") { close(); document.removeEventListener("keydown", esc); }
  });
  document.body.appendChild(overlay);
  return { overlay, close };
}

function field(labelText, control) {
  return el("div.field", {}, [el("label", { text: labelText }), control]);
}
function btn(text, cls, onClick) {
  return el("button.btn" + (cls ? "." + cls : ""), { onclick: onClick, text });
}

// --- Lock dialog ------------------------------------------------------------
export function lockDialog(targets) {
  const msg = el("textarea.textarea", { placeholder: t("lock_msg_ph") });
  const m = modal({
    title: t("lock_title"),
    sub: targets.length + " " + t("computers"),
    body: [field(t("message"), msg)],
    actions: [
      btn(t("cancel"), "ghost", () => m.close()),
      btn(t("lock"), "primary", () => {
        send("lock", { targets, title: t("lock_default_title"), message: msg.value });
        toast(t("lock"), targets.length + " " + t("toast_locked"), "ok");
        m.close();
      }),
    ],
  });
  msg.focus();
}

// --- Message dialog ---------------------------------------------------------
export function messageDialog(targets) {
  const title = el("input.input", { placeholder: t("msg_title_ph"), value: "SmartEYE" });
  const bodyI = el("textarea.textarea", { placeholder: t("msg_body_ph") });
  const m = modal({
    title: t("send_message"),
    sub: targets.length + " " + t("computers"),
    body: [field(t("msg_title_ph"), title), field(t("msg_body_ph"), bodyI)],
    actions: [
      btn(t("cancel"), "ghost", () => m.close()),
      btn(t("send"), "primary", () => {
        send("message", { targets, title: title.value, body: bodyI.value, timeout: 0 });
        toast(t("message"), t("toast_sent"), "ok");
        m.close();
      }),
    ],
  });
  bodyI.focus();
}

// --- Launch dialog ----------------------------------------------------------
export function launchDialog(targets) {
  const target = el("input.input", { placeholder: t("target_ph") });
  const isURL = el("input", { type: "checkbox", checked: true, style: "width:auto" });
  const m = modal({
    title: t("open_app"),
    sub: targets.length + " " + t("computers"),
    body: [
      field(t("launch"), target),
      el("label", { style: "display:flex;gap:8px;align-items:center;cursor:pointer;color:var(--text-soft)" }, [
        isURL, document.createTextNode(" " + t("is_url")),
      ]),
    ],
    actions: [
      btn(t("cancel"), "ghost", () => m.close()),
      btn(t("launch"), "primary", () => {
        if (!target.value.trim()) return;
        send("launch", { targets, target: target.value.trim(), is_url: isURL.checked });
        toast(t("launch"), t("toast_sent"), "ok");
        m.close();
      }),
    ],
  });
  target.focus();
}

// --- Room create / rename ---------------------------------------------------
export function roomDialog(existing) {
  const name = el("input.input", { placeholder: t("room_name_ph"), value: existing ? existing.name : "" });
  const m = modal({
    title: existing ? t("rename") : t("new_room"),
    body: [field(t("room_name"), name)],
    actions: [
      btn(t("cancel"), "ghost", () => m.close()),
      btn(existing ? t("apply") : t("create"), "primary", () => {
        if (!name.value.trim()) return;
        if (existing) send("rename_room", { id: existing.id, name: name.value.trim() });
        else send("create_room", { name: name.value.trim() });
        m.close();
      }),
    ],
  });
  name.focus();
  name.addEventListener("keydown", (e) => { if (e.key === "Enter") m.overlay.querySelector(".btn.primary").click(); });
}

// --- Assign to room ---------------------------------------------------------
export function assignDialog(targets, rooms) {
  const sel = el("select.select", {}, [
    el("option", { value: "" , text: t("no_room") }),
    ...rooms.map((r) => el("option", { value: r.id, text: r.name })),
  ]);
  const m = modal({
    title: t("assign_room"),
    sub: targets.length + " " + t("computers"),
    body: [field(t("choose_room"), sel)],
    actions: [
      btn(t("cancel"), "ghost", () => m.close()),
      btn(t("apply"), "primary", () => {
        send("assign_room", { devices: targets, room: sel.value });
        m.close();
      }),
    ],
  });
}

// --- Power menu (dropdown) ---------------------------------------------------
export function powerMenu(x, y, targets) {
  closeMenus();
  const item = (ic, label, action, danger) =>
    el("button" + (danger ? ".danger" : ""), {
      html: icon(ic) + "<span>" + label + "</span>",
      onclick: () => {
        send("power", { targets, action, delay: action === "shutdown" || action === "reboot" ? 3 : 0 });
        toast(t("power"), t("toast_sent"), danger ? "danger" : "ok");
        closeMenus();
      },
    });
  const menu = el("div.menu", {}, [
    item("power", t("shutdown"), "shutdown", true),
    item("reboot", t("reboot"), "reboot"),
    item("logout", t("logoff"), "logoff"),
    item("moon", t("sleep"), "sleep"),
  ]);
  positionMenu(menu, x, y);
}

// --- Room context menu ------------------------------------------------------
export function roomMenu(x, y, room) {
  closeMenus();
  const menu = el("div.menu", {}, [
    el("button", { html: icon("edit") + "<span>" + t("rename") + "</span>", onclick: () => { roomDialog(room); closeMenus(); } }),
    el("div.msep"),
    el("button.danger", { html: icon("trash") + "<span>" + t("delete") + "</span>", onclick: () => { send("delete_room", { id: room.id }); closeMenus(); } }),
  ]);
  positionMenu(menu, x, y);
}

function positionMenu(menu, x, y) {
  menu.style.left = x + "px";
  menu.style.top = y + "px";
  document.body.appendChild(menu);
  const r = menu.getBoundingClientRect();
  if (r.right > innerWidth) menu.style.left = innerWidth - r.width - 8 + "px";
  if (r.bottom > innerHeight) menu.style.top = y - r.height + "px";
  setTimeout(() => document.addEventListener("mousedown", onDoc), 0);
}
function onDoc(e) { if (!e.target.closest(".menu")) closeMenus(); }
export function closeMenus() {
  document.querySelectorAll(".menu").forEach((m) => m.remove());
  document.removeEventListener("mousedown", onDoc);
}
