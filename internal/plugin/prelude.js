// The host's side of a plugin's world, run as a script in its VM before the
// plugin. It turns three host functions that pass strings (JSON) into the
// dalibre object, and calls the plugin's exports for the host, reporting
// each outcome back as JSON. The host functions are taken off the global
// object, so only this code can call them.
(() => {
  "use strict";
  const hostFetch = __host_fetch, hostLog = __host_log, hostSettle = __host_settle, hostInfo = __host_info;
  for (const k of ["__host_fetch", "__host_log", "__host_settle", "__host_info"]) delete globalThis[k];
  const { parse, stringify } = JSON;
  const { freeze, defineProperty, keys } = Object;
  const resolved = Promise.resolve.bind(Promise), rejected = Promise.reject.bind(Promise);

  const { api, plugin, settings } = parse(hostInfo());

  function hostError(e) {
    const err = new Error(e.message);
    err.kind = e.kind;
    return err;
  }

  const log = (level) => (...args) => {
    hostLog(level, args.map((a) => {
      if (typeof a === "string") return a;
      try { return a instanceof Error ? String(a) : stringify(a) ?? String(a); } catch { return String(a); }
    }).join(" "));
  };

  const dalibre = freeze({
    api,
    plugin: freeze(plugin),
    settings: freeze(settings),
    fetch(url, init) {
      let r;
      try {
        r = parse(hostFetch(String(url), stringify(init ?? {})));
      } catch (e) {
        return rejected(e);
      }
      if (r.error) return rejected(hostError(r.error));
      const body = r.body;
      return resolved(freeze({
        status: r.status, statusText: r.statusText, ok: r.ok, url: r.url, headers: freeze(r.headers),
        text: () => body,
        json: () => parse(body),
      }));
    },
    error(kind, message) {
      const e = new Error(message === undefined ? String(kind) : String(message));
      e.name = "DalibreError";
      e.kind = String(kind);
      return e;
    },
    log: log("log"),
  });
  const fixed = (name, value) => defineProperty(globalThis, name, { value, writable: false, configurable: false });
  fixed("dalibre", dalibre);
  fixed("console", freeze({ log: log("log"), info: log("info"), warn: log("warn"), error: log("error"), debug: log("debug") }));

  // CommonJS, for plugins that set module.exports or exports.x.
  globalThis.module = { exports: {} };
  globalThis.exports = globalThis.module.exports;

  function describe(e) {
    if (e instanceof Error) {
      return { name: String(e.name), message: String(e.message), stack: String(e.stack ?? ""), kind: typeof e.kind === "string" ? e.kind : "" };
    }
    let message;
    try { message = String(e); } catch { message = "a value that isn’t an Error"; }
    return { name: "", message, stack: "", kind: "" };
  }

  function report(promise) {
    promise.then((value) => {
      let s;
      try {
        s = stringify({ ok: true, value: value === undefined ? null : value });
      } catch (e) {
        s = stringify({ ok: false, error: { name: "", kind: "invalid", stack: "", message: "returned what isn’t JSON: " + describe(e).message } });
      }
      hostSettle(s);
    }, (e) => hostSettle(stringify({ ok: false, error: describe(e) })));
  }

  let exported = {};

  // __dalibre_load imports the plugin's main module and reports the names
  // of the functions it exports: ES module exports, else an exported
  // default object, else module.exports.
  fixed("__dalibre_load", (main) => {
    report(import("./" + main).then((m) => {
      const named = keys(m).filter((k) => k !== "default");
      if (named.length > 0) exported = m;
      else if (m.default && typeof m.default === "object") exported = m.default;
      else exported = globalThis.module.exports ?? {};
      return keys(exported).filter((k) => typeof exported[k] === "function");
    }));
  });

  // __dalibre_call calls an export with arguments given as a JSON array.
  fixed("__dalibre_call", (name, args) => {
    let p;
    try {
      p = resolved(exported[name](...parse(args)));
    } catch (e) {
      p = rejected(e);
    }
    report(p);
  });
})();
