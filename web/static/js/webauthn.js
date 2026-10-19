// Security key ceremonies (spec 2026-10-19-manager-mfa). A button carrying
// data-begin-url and data-finish-url asks the server for options, runs
// navigator.credentials.create (options with a user: registration) or .get
// (login), posts the credential as JSON and follows {"redirect": ...}; any
// {"error": ...} is shown in the nearest [data-webauthn-error] element.
{
  const b64urlToBuffer = (s) => {
    const b64 = s.replace(/-/g, "+").replace(/_/g, "/").padEnd(Math.ceil(s.length / 4) * 4, "=");
    const bin = atob(b64);
    const out = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
    return out.buffer;
  };
  const bufferToB64url = (buf) => {
    const bytes = new Uint8Array(buf);
    let bin = "";
    for (const b of bytes) bin += String.fromCharCode(b);
    return btoa(bin).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
  };

  const errorSlot = (btn) =>
    (btn.parentElement && btn.parentElement.querySelector("[data-webauthn-error]")) ||
    document.querySelector("[data-webauthn-error]");

  const showError = (btn, msg) => {
    const slot = errorSlot(btn);
    if (!slot) return;
    slot.textContent = msg;
    slot.hidden = false;
  };

  const post = async (url, body) => {
    const headers = { "Content-Type": "application/json", Accept: "application/json" };
    const csrf = document.querySelector('meta[name="csrf-token"]');
    if (csrf) headers["X-CSRF-Token"] = csrf.content;
    const res = await fetch(url, { method: "POST", headers, body, credentials: "same-origin" });
    let data = {};
    try {
      data = await res.json();
    } catch {
      data = {};
    }
    if (!res.ok) throw new Error(data.error || "Something went wrong, please try again.");
    return data;
  };

  // The server sends the WebAuthn options as JSON with base64url binaries.
  const decodeOptions = (pk) => {
    pk.challenge = b64urlToBuffer(pk.challenge);
    if (pk.user) pk.user.id = b64urlToBuffer(pk.user.id);
    for (const list of [pk.excludeCredentials, pk.allowCredentials]) {
      for (const c of list || []) c.id = b64urlToBuffer(c.id);
    }
    return pk;
  };

  const encodeCredential = (cred) => {
    const r = cred.response;
    const response = { clientDataJSON: bufferToB64url(r.clientDataJSON) };
    if (r.attestationObject) {
      response.attestationObject = bufferToB64url(r.attestationObject);
      if (r.getTransports) response.transports = r.getTransports();
    } else {
      response.authenticatorData = bufferToB64url(r.authenticatorData);
      response.signature = bufferToB64url(r.signature);
      if (r.userHandle) response.userHandle = bufferToB64url(r.userHandle);
    }
    return {
      id: cred.id,
      rawId: bufferToB64url(cred.rawId),
      type: cred.type,
      authenticatorAttachment: cred.authenticatorAttachment || undefined,
      clientExtensionResults: cred.getClientExtensionResults ? cred.getClientExtensionResults() : {},
      response,
    };
  };

  const run = async (btn) => {
    const slot = errorSlot(btn);
    if (slot) slot.hidden = true;
    btn.disabled = true;
    try {
      const options = await post(btn.dataset.beginUrl, "{}");
      const pk = decodeOptions(options.publicKey);
      const cred = pk.user
        ? await navigator.credentials.create({ publicKey: pk })
        : await navigator.credentials.get({ publicKey: pk });
      if (!cred) throw new Error("No security key answered");
      const done = await post(btn.dataset.finishUrl, JSON.stringify(encodeCredential(cred)));
      if (done.redirect) {
        window.location.assign(done.redirect);
        return;
      }
    } catch (err) {
      // NotAllowedError: the user cancelled or the prompt timed out.
      showError(btn, err && err.name === "NotAllowedError" ? "The security key request was cancelled or timed out" : err.message);
    }
    btn.disabled = false;
  };

  for (const btn of document.querySelectorAll("button[data-begin-url][data-finish-url]")) {
    if (typeof window.PublicKeyCredential === "undefined") {
      btn.disabled = true;
      showError(btn, "This browser does not support security keys");
      continue;
    }
    btn.addEventListener("click", () => run(btn));
  }
}
