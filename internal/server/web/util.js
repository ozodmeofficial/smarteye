// Tiny DOM helpers.
export const $ = (sel, root = document) => root.querySelector(sel);
export const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));

// el("div.cls#id", {attr}, [children|string]) -> HTMLElement
export function el(tag, attrs = {}, children = []) {
  let name = "div", id = null, cls = [];
  tag.replace(/([.#]?[^.#]+)/g, (m) => {
    if (m[0] === ".") cls.push(m.slice(1));
    else if (m[0] === "#") id = m.slice(1);
    else name = m;
  });
  const e = document.createElement(name);
  if (id) e.id = id;
  if (cls.length) e.className = cls.join(" ");
  for (const [k, v] of Object.entries(attrs)) {
    if (k === "html") e.innerHTML = v;
    else if (k === "text") e.textContent = v;
    else if (k.startsWith("on") && typeof v === "function") e.addEventListener(k.slice(2), v);
    else if (v !== null && v !== undefined && v !== false) e.setAttribute(k, v === true ? "" : v);
  }
  for (const c of [].concat(children)) {
    if (c == null) continue;
    e.appendChild(typeof c === "string" ? document.createTextNode(c) : c);
  }
  return e;
}

export function esc(s) {
  return String(s ?? "").replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}
