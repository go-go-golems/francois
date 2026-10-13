// Browser-bound launch exchange between the trusted portal (parent) and an
// instance origin (child frame). postMessage only carries a one-use code; the
// server binds it to the portal session, instance, origin and S256 challenge.
export type Grant = { grant: string; expiresAt: string };

const b64url = (bytes: Uint8Array) =>
  btoa(String.fromCharCode(...bytes))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");

export async function pkce() {
  const verifier = b64url(crypto.getRandomValues(new Uint8Array(32)));
  const digest = await crypto.subtle.digest(
    "SHA-256",
    new TextEncoder().encode(verifier),
  );
  return { verifier, challenge: b64url(new Uint8Array(digest)) };
}

// requestCode asks the parent portal for a launch code for this challenge.
export function requestCode(
  portal: string,
  challenge: string,
  purpose: "launch" | "renew",
): Promise<string> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      removeEventListener("message", receive);
      reject({ code: "launch_timeout", message: "The portal did not answer" });
    }, 20000);
    function receive(e: MessageEvent) {
      if (e.origin !== portal || e.source !== parent) return;
      const d = e.data;
      if (!d || d.challenge !== challenge) return;
      if (d.type === "francois-launch-code" && typeof d.code === "string") {
        clearTimeout(timer);
        removeEventListener("message", receive);
        resolve(d.code);
      } else if (d.type === "francois-launch-denied") {
        clearTimeout(timer);
        removeEventListener("message", receive);
        reject({ code: d.code, message: d.message });
      }
    }
    addEventListener("message", receive);
    parent.postMessage({ type: "francois-launch-ready", challenge, purpose }, portal);
  });
}

export async function exchange(code: string, verifier: string): Promise<Grant> {
  const r = await fetch("/api/launch/exchange", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code, verifier }),
  });
  const body = await r.json();
  if (!r.ok) throw body.error;
  return body;
}

export async function obtainGrant(
  portal: string,
  purpose: "launch" | "renew",
): Promise<Grant> {
  const { verifier, challenge } = await pkce();
  const code = await requestCode(portal, challenge, purpose);
  return exchange(code, verifier);
}
