// Runs at document-start in Discord frames, before Discord's bundle.
(() => {
  const send = (t) => {
    if (typeof t !== "string") return;
    const clean = t.replace(/^"|"$/g, "").trim();
    if (!clean) return;
    window.webkit?.messageHandlers?.dissentToken?.postMessage(clean);
  };

  // Discord persists the token as localStorage["token"] (JSON string) after
  // login (password, MFA, QR, etc.), then deletes window.localStorage.
  // Hooking Storage.prototype survives that deletion.
  const setItem = Storage.prototype.setItem;
  Storage.prototype.setItem = function (k, v) {
    if (k === "token") {
      send(v);
    }
    return setItem.apply(this, arguments);
  };

  // Also watch login/MFA response bodies off fetch and XHR.
  const authPath = /\/api\/v\d+\/auth\/(login|mfa\/|remote-auth\/)/;

  const origFetch = window.fetch;
  window.fetch = async function (...args) {
    const res = await origFetch.apply(this, args);
    try {
      const url = String(args[0]?.url ?? args[0]);
      if (authPath.test(url)) {
        res.clone().json().then((j) => send(j?.token)).catch(() => {});
      }
    } catch {}
    return res;
  };

  const origOpen = XMLHttpRequest.prototype.open;
  const origSend = XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.open = function (method, url) {
    this.__dissentURL = String(url);
    return origOpen.apply(this, arguments);
  };
  XMLHttpRequest.prototype.send = function () {
    if (this.__dissentURL && authPath.test(this.__dissentURL)) {
      this.addEventListener("load", () => {
        try {
          const j = JSON.parse(this.responseText);
          send(j?.token);
        } catch {}
      });
    }
    return origSend.apply(this, arguments);
  };
})();
