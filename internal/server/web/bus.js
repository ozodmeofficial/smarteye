// Shared event/command bus so dialogs and the control window can talk to the
// WebSocket layer without importing app.js (which would create a cycle).
const handlers = {};
let sender = () => {};

export function setSender(fn) { sender = fn; }
// send(type, payload) delivers a command to the server over the WebSocket.
export function send(type, payload) { sender(type, payload || {}); }

export function on(event, fn) { (handlers[event] ||= []).push(fn); }
export function emit(event, data) { (handlers[event] || []).forEach((f) => f(data)); }
