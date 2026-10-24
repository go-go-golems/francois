import { bindInstanceLaunch } from "./instance-launch";
import React, { useEffect, useRef, useState } from "react";
import { api, errorMessage, Instance } from "./api";
import { navigate, store } from "./store";
import { date, Empty, Link, loginHref, reasonLabel } from "./ui";

export function Instances() {
  const { data: session } = api.useSessionQuery();
  const { data: list = [], isLoading } = api.useInstancesQuery(undefined, {
    skip: !session?.authenticated,
  });
  if (session && !session.authenticated)
    return (
      <main className="lost">
        <h1>Sign in to see your instances.</h1>
        <a className="btn btn-ink" href={loginHref("/instances/")}>
          Sign in
        </a>
      </main>
    );
  const own = list.filter((i) => i.role === "owner");
  const shared = list.filter((i) => i.role !== "owner");
  return (
    <main className="library">
      <div className="lib-head">
        <h2>Instances</h2>
        <Link className="u" to="/">
          Start one from the catalog
        </Link>
      </div>
      {session?.access && !session.access.allowed && (
        <p className="notice bad">
          {reasonLabel(session.access.reason)} — instances are frozen until
          access is active.{" "}
          <Link className="u" to="/account">
            Account
          </Link>
        </p>
      )}
      {isLoading ? (
        <Empty>Loading…</Empty>
      ) : (
        <InstanceList
          list={own}
          empty="No instances yet. Run an application from the catalog."
        />
      )}
      {shared.length > 0 && (
        <>
          <div className="lib-head">
            <h2>Shared with you</h2>
          </div>
          <InstanceList list={shared} empty="" />
        </>
      )}
    </main>
  );
}

function InstanceList({ list, empty }: { list: Instance[]; empty: string }) {
  if (!list.length) return <Empty>{empty}</Empty>;
  return (
    <div className="lib-list">
      {list.map((i) => (
        <div
          className={"lib-row" + (i.state !== "active" ? " is-unlisted" : "")}
          key={i.id}
        >
          <div className="lib-thumb mono-mini">
            {i.title.slice(0, 2).toUpperCase()}
          </div>
          <div className="lib-main">
            <Link to={"/instances/" + i.id}>
              <strong>{i.title}</strong>
            </Link>
            <span className="muted small">
              {i.artifactId} v{i.version} · owner {i.ownerName || "—"}
            </span>
          </div>
          <span className="lib-tags">
            {i.members} {i.members === 1 ? "member" : "members"}
          </span>
          <span
            className={"lib-status" + (i.state === "active" ? " on" : " bad")}
          >
            {i.state === "active" ? i.role : "suspended"}
          </span>
          <span className="lib-added">{date(i.updatedAt)}</span>
          <div className="lib-actions">
            <Link className="btn btn-soft" to={"/instances/" + i.id}>
              Open
            </Link>
          </div>
        </div>
      ))}
    </div>
  );
}

// Launcher hosts the instance in a sandboxed frame on its own origin and
// answers only the narrow launch/renew handshake from that exact frame.
export function Launcher({ id }: { id: string }) {
  const { data: inst, isLoading, error } = api.useInstanceQuery(id);
  // The launch handshake is a CSRF-protected mutation; wait for the token.
  const { data: session } = api.useSessionQuery();
  const ready = !!session?.csrfToken;
  const [launch] = api.useLaunchMutation();
  const frame = useRef<HTMLIFrameElement>(null);
  const [denied, setDenied] = useState("");
  const origin = inst?.origin;
  useEffect(() => {
    if (!inst?.artifactTitle || !session?.brand) return;
    const previous = document.title;
    document.title = `${inst.artifactTitle} · ${session.brand}`;
    return () => {
      document.title = previous;
    };
  }, [inst?.artifactTitle, session?.brand]);
  useEffect(() => {
    if (!origin) return;
    if (!frame.current) return;
    return bindInstanceLaunch(
      frame.current,
      origin,
      (challenge) => launch({ id, challenge }).unwrap(),
      setDenied,
    );
  }, [origin, id, ready, launch]);
  useEffect(() => {
    const esc = (e: KeyboardEvent) => {
      if (e.key === "Escape" && document.activeElement?.tagName !== "IFRAME")
        navigate("/");
    };
    addEventListener("keydown", esc);
    document.body.classList.add("has-window");
    return () => {
      removeEventListener("keydown", esc);
      document.body.classList.remove("has-window");
    };
  }, []);
  if (isLoading)
    return (
      <main>
        <Empty>Loading…</Empty>
      </main>
    );
  if (error || !inst)
    return (
      <main className="lost">
        <h1>Instance not found.</h1>
        <Link className="u" to="/instances/">
          Your instances
        </Link>
      </main>
    );
  return (
    <div className="viewer">
      <Link
        className="viewer-close u"
        to={inst.kind === "preview" ? "/studio" : "/apps/" + inst.artifactId}
        aria-label="Close application"
      >
        Close
      </Link>
      <div className="v-table">
        {inst.launchBlocked ? (
          <div className="v-blocked">
            <p>{inst.launchBlocked}</p>
            <Link className="u" to="/">
              Back to the site
            </Link>
          </div>
        ) : !ready ? null : (
          <div className="v-device">
            <iframe
              ref={frame}
              title={inst.title}
              src={inst.origin + "/launch"}
              sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-modals allow-downloads"
              allow="clipboard-write"
            />
          </div>
        )}
        {denied && !inst.launchBlocked && <p className="v-denied">{denied}</p>}
      </div>
    </div>
  );
}

// openedHere tells a tab the storefront opened from one opened some other
// way. /instances/new starts an instance on load, so a link to it from
// elsewhere must not create instances for whoever follows it.
function openedHere() {
  try {
    return window.opener?.location.origin === location.origin;
  } catch {
    return false; // a cross-origin opener
  }
}

// Starter is the new tab "Run" opens. It shows the application starting at
// once, instead of a blank tab, creates (or resumes) the instance and then
// becomes that instance's page.
export function Starter() {
  const q = new URLSearchParams(location.search);
  const artifact = q.get("artifact") ?? "";
  const release = q.get("release") ?? "";
  const title = q.get("title") ?? "";
  const demo = q.get("demo") === "1";
  const back = artifact ? "/apps/" + encodeURIComponent(artifact) : "/";
  const { data: session } = api.useSessionQuery();
  const [openInstance] = api.useOpenInstanceMutation();
  const [startDemo] = api.useStartDemoMutation();
  const [failed, setFailed] = useState("");
  const started = useRef(false);
  const ready = !!session?.csrfToken || session?.authenticated === false;
  useEffect(() => {
    if (title) document.title = "Starting " + title;
    document.body.classList.add("has-window");
    return () => document.body.classList.remove("has-window");
  }, [title]);
  useEffect(() => {
    if (!ready || started.current) return;
    started.current = true;
    if (!openedHere()) {
      location.replace(back);
      return;
    }
    (async () => {
      try {
        const { id } = demo
          ? await startDemo({ artifactId: artifact }).unwrap()
          : await openInstance({ releaseId: release, title }).unwrap();
        // A first demo signs the visitor in as a guest: the launch handshake
        // needs that session's CSRF token.
        await store.dispatch(
          api.endpoints.session.initiate(undefined, { forceRefetch: true }),
        );
        navigate("/instances/" + id, true);
      } catch (e) {
        setFailed(errorMessage(e));
      }
    })();
  }, [ready, demo, artifact, release, title, back, openInstance, startDemo]);
  return (
    <div className="viewer">
      <a className="viewer-close u" href={back} aria-label="Close application">
        Close
      </a>
      <div className="v-table">
        <div className="v-blocked">
          {failed ? (
            <>
              <p>{failed}</p>
              <a className="u" href={back}>
                Back to the application
              </a>
            </>
          ) : (
            <p>Starting {title || "the application"}…</p>
          )}
        </div>
      </div>
    </div>
  );
}
