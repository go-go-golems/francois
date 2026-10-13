import React, { useEffect, useRef, useState } from "react";
import { useDispatch } from "react-redux";
import { api, errorMessage, Instance } from "./api";
import { navigate, show } from "./store";
import { date, Empty, Line, Link, loginHref, reasonLabel } from "./ui";

export function Instances() {
  const { data: session } = api.useSessionQuery();
  const { data: list = [], isLoading } = api.useInstancesQuery(undefined, { skip: !session?.authenticated });
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
          {reasonLabel(session.access.reason)} — instances are frozen until access is active.{" "}
          <Link className="u" to="/account">
            Account
          </Link>
        </p>
      )}
      {isLoading ? <Empty>Loading…</Empty> : <InstanceList list={own} empty="No instances yet. Run an application from the catalog." />}
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
        <div className={"lib-row" + (i.state !== "active" ? " is-unlisted" : "")} key={i.id}>
          <div className="lib-thumb mono-mini">{i.title.slice(0, 2).toUpperCase()}</div>
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
          <span className={"lib-status" + (i.state === "active" ? " on" : " bad")}>{i.state === "active" ? i.role : "suspended"}</span>
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
  const [launch] = api.useLaunchMutation();
  const frame = useRef<HTMLIFrameElement>(null);
  const [info, setInfo] = useState(false);
  const [denied, setDenied] = useState("");
  const [nonce, setNonce] = useState(0);
  const origin = inst?.origin;
  useEffect(() => {
    if (!origin) return;
    const receive = async (e: MessageEvent) => {
      if (e.origin !== origin || e.source !== frame.current?.contentWindow) return;
      const d = e.data;
      if (d?.type !== "francois-launch-ready" || typeof d.challenge !== "string") return;
      const target = frame.current!.contentWindow!;
      try {
        const r = await launch({ id, challenge: d.challenge }).unwrap();
        setDenied("");
        target.postMessage({ type: "francois-launch-code", code: r.code, challenge: d.challenge }, origin);
      } catch (err: any) {
        const message = errorMessage(err, "This instance cannot be opened right now");
        setDenied(message);
        target.postMessage({ type: "francois-launch-denied", code: err?.data?.error?.code ?? "denied", message, challenge: d.challenge }, origin);
      }
    };
    addEventListener("message", receive);
    return () => removeEventListener("message", receive);
  }, [origin, id]);
  useEffect(() => {
    const esc = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !info && document.activeElement?.tagName !== "IFRAME") navigate("/instances/");
    };
    addEventListener("keydown", esc);
    document.body.classList.add("has-window");
    return () => {
      removeEventListener("keydown", esc);
      document.body.classList.remove("has-window");
    };
  }, [info]);
  if (isLoading) return <main><Empty>Loading…</Empty></main>;
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
      <div className="v-top">
        <div className="v-title">
          <strong>{inst.title}</strong>
          <span className="v-desc">
            {inst.artifactId} v{inst.version} · {inst.kind === "preview" ? "admin preview" : inst.role} · <span className="mono">{inst.origin.replace(/^https?:\/\//, "")}</span>
          </span>
        </div>
        <div className="v-actions">
          <button className="tlink plain" onClick={() => setNonce(nonce + 1)}>
            Reload
          </button>
          <button className="tlink plain" aria-pressed={info} onClick={() => setInfo(!info)}>
            {inst.role === "owner" && inst.kind === "workspace" ? "Share & info" : "Info"}
          </button>
          <Link className="tlink plain" to={inst.kind === "preview" ? "/admin/releases" : "/instances/"}>
            Close <span className="muted">esc</span>
          </Link>
        </div>
      </div>
      <div className="v-table">
        {inst.launchBlocked ? (
          <div className="v-blocked">
            <p>{inst.launchBlocked}</p>
            <Link className="u" to="/account">
              Account
            </Link>
          </div>
        ) : (
          <div className="v-device">
            <iframe
              key={nonce}
              ref={frame}
              title={inst.title}
              src={inst.origin + "/launch"}
              sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-modals allow-downloads"
              allow="clipboard-write"
            />
          </div>
        )}
        {denied && !inst.launchBlocked && <p className="v-denied">{denied}</p>}
        {info && <InstanceInfo inst={inst} onClose={() => setInfo(false)} />}
      </div>
    </div>
  );
}

function InstanceInfo({ inst, onClose }: { inst: Instance; onClose: () => void }) {
  const [add, { isLoading: adding }] = api.useAddMemberMutation();
  const [remove] = api.useRemoveMemberMutation();
  const [upgrade] = api.useUpgradeMutation();
  const [rename] = api.useRenameInstanceMutation();
  const [del] = api.useDeleteInstanceMutation();
  const [email, setEmail] = useState("");
  const [role, setRole] = useState("editor");
  const [title, setTitle] = useState(inst.title);
  const dispatch = useDispatch();
  const owner = inst.role === "owner" && inst.kind === "workspace";
  const act = async (p: Promise<unknown>, ok: string) => {
    try {
      await p;
      dispatch(show({ text: ok }));
    } catch (e) {
      dispatch(show({ text: errorMessage(e), bad: true }));
    }
  };
  return (
    <aside className="v-info" aria-label="Instance information">
      <div className="v-info-head">
        <strong>{owner ? "Share & info" : "Info"}</strong>
        <button className="tlink quiet" onClick={onClose}>
          Close
        </button>
      </div>
      <p className="v-info-desc">Private by default. Members must be registered subscribers; each runs model calls on their own allowance.</p>
      <div className="v-list">
        <Line k="Application">
          {inst.artifactId} v{inst.version}
        </Line>
        <Line k="Your role">{inst.role}</Line>
        <Line k="Owner">{inst.ownerName}</Line>
        <Line k="Origin" mono>
          {inst.origin.replace(/^https?:\/\//, "")}
        </Line>
        <Line k="Created">{date(inst.createdAt)}</Line>
      </div>
      <div className="v-list">
        {(inst.membersList ?? []).map((m) => (
          <div key={m.userId}>
            <span>
              {m.name || m.email || m.userId} <span className="muted">{m.email && m.name ? m.email : ""}</span>
            </span>
            <span>
              {m.role}
              {owner && m.role !== "owner" && (
                <button className="tlink quiet" onClick={() => act(remove({ id: inst.id, userId: m.userId }).unwrap(), "Member removed")}>
                  {" "}
                  remove
                </button>
              )}
            </span>
          </div>
        ))}
      </div>
      {owner && (
        <form
          className="form tight"
          onSubmit={(e) => {
            e.preventDefault();
            void act(add({ id: inst.id, email, role }).unwrap(), "Shared with " + email).then(() => setEmail(""));
          }}
        >
          <div className="field">
            <label htmlFor="share-email">Share with a subscriber</label>
            <input id="share-email" type="email" required placeholder="name@example.com" value={email} onChange={(e) => setEmail(e.target.value)} />
          </div>
          <div className="seg" role="group" aria-label="Role">
            {["editor", "viewer"].map((r) => (
              <button type="button" key={r} aria-pressed={role === r} onClick={() => setRole(r)}>
                {r}
              </button>
            ))}
          </div>
          <button className="btn btn-ink" disabled={adding}>
            Share
          </button>
        </form>
      )}
      {owner && (inst.upgrades?.length ?? 0) > 0 && (
        <div className="v-list">
          {inst.upgrades!.map((r) => (
            <div key={r.id}>
              <span>
                Upgrade to v{r.version} <span className="muted">{r.changelog.slice(0, 60)}</span>
              </span>
              <button className="tlink" onClick={() => act(upgrade({ id: inst.id, releaseId: r.id }).unwrap(), "Upgraded; data preserved")}>
                Upgrade
              </button>
            </div>
          ))}
        </div>
      )}
      {owner && (
        <form
          className="form tight"
          onSubmit={(e) => {
            e.preventDefault();
            void act(rename({ id: inst.id, title }).unwrap(), "Renamed");
          }}
        >
          <div className="field">
            <label htmlFor="inst-title">Title</label>
            <input id="inst-title" value={title} onChange={(e) => setTitle(e.target.value)} />
          </div>
          <div className="form-foot">
            <button className="btn btn-soft">Rename</button>
            <button
              type="button"
              className="tlink quiet danger"
              onClick={async () => {
                if (!confirm("Delete this instance and all of its data? This cannot be undone.")) return;
                await act(del(inst.id).unwrap(), "Instance deleted");
                navigate("/instances/");
              }}
            >
              Delete instance
            </button>
          </div>
        </form>
      )}
    </aside>
  );
}
