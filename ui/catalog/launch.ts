// Trusted bootstrap served at <instance origin>/launch. It holds no paid
// bytes: it obtains a scoped grant from the parent portal, then fetches the
// authored page (with the runtime injected) and replaces this document.
import { obtainGrant } from "./launch-protocol";

const portal = document.documentElement.dataset.portal!;
const status = document.getElementById("francois-launch")!;

async function launch() {
  if (parent === window) {
    status.textContent = "Open this application from your Francois account.";
    return;
  }
  try {
    const { grant } = await obtainGrant(portal, "launch");
    const r = await fetch("/page", { headers: { Authorization: "Bearer " + grant } });
    if (!r.ok) throw (await r.json()).error;
    const html = await r.text();
    document.open();
    document.write(html);
    document.close();
  } catch (e: any) {
    status.textContent = e?.message ?? "Could not start this application.";
    status.className = "failed";
  }
}
void launch();
