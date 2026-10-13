import React, { useMemo, useState } from "react";
import { useDispatch } from "react-redux";
import { api, CatalogEntry, errorMessage, Release } from "./api";
import { navigate, show } from "./store";
import { bytes, date, Empty, Link, loginHref, Monogram } from "./ui";

// useRun creates a private instance pinned to a release and opens it.
export function useRun() {
  const { data: session } = api.useSessionQuery();
  const [create, { isLoading }] = api.useCreateInstanceMutation();
  const dispatch = useDispatch();
  const run = async (release: Release, title?: string) => {
    if (!session?.authenticated) {
      location.href = loginHref("/apps/" + release.artifactId);
      return;
    }
    if (!session.access?.allowed) {
      navigate("/account?subscribe=1");
      return;
    }
    try {
      const inst = await create({ releaseId: release.id, title }).unwrap();
      navigate("/instances/" + inst.id);
    } catch (e) {
      dispatch(show({ text: errorMessage(e, "Could not start an instance"), bad: true }));
    }
  };
  return { run, busy: isLoading };
}

function downloadable(release: Release, allowed: boolean) {
  return release.downloadAccess === "free_sample" || allowed;
}

export function Catalog() {
  const { data: session } = api.useSessionQuery();
  const { data: entries = [], isLoading, error } = api.useCatalogQuery();
  const [tag, setTag] = useState("");
  const [sort, setSort] = useState("new");
  const { run, busy } = useRun();
  const tags = useMemo(() => {
    const counts = new Map<string, number>();
    entries.forEach((e) => e.tags.forEach((t) => counts.set(t, (counts.get(t) ?? 0) + 1)));
    return [...counts.entries()].sort((a, b) => b[1] - a[1]);
  }, [entries]);
  const list = entries
    .filter((e) => !tag || e.tags.includes(tag))
    .sort((a, b) => (sort === "az" ? a.title.localeCompare(b.title) : b.updatedAt.localeCompare(a.updatedAt)));
  const allowed = session?.access?.allowed === true;
  const params = new URLSearchParams(location.search);
  return (
    <main>
      <section className="hero">
        <h1>
          Single-file HTML applications, hosted for you: a private origin, a real database, shared rooms and model calls{" "}
          <span className="muted">— no setup, no keys.</span>
        </h1>
      </section>
      {params.get("login_error") && (
        <p className="notice bad">Sign-in did not complete ({params.get("login_error")}). Please try again.</p>
      )}
      <section className="controls">
        <div className="views">
          <label>
            <span className="visually-hidden">Sort</span>
            <select className="sort" value={sort} onChange={(e) => setSort(e.target.value)}>
              <option value="new">Newest first</option>
              <option value="az">A–Z</option>
            </select>
          </label>
          <span className="result-count">
            {list.length} {list.length === 1 ? "application" : "applications"}
          </span>
          {session?.authenticated && !allowed && (
            <Link className="u" to="/account?subscribe=1">
              Subscribe to run applications
            </Link>
          )}
          {!session?.authenticated && session?.loginAvailable && (
            <a className="u" href={loginHref("/")}>
              Sign in to run applications
            </a>
          )}
        </div>
        {tags.length > 0 && (
          <div className="tags" role="group" aria-label="Filter by tag">
            <button className="tag" aria-pressed={tag === ""} onClick={() => setTag("")}>
              <span className="tag-label">all</span>
            </button>
            {tags.map(([t, n]) => (
              <button key={t} className="tag" aria-pressed={tag === t} onClick={() => setTag(tag === t ? "" : t)}>
                <span className="tag-label">{t}</span> <span className="tag-count">{n}</span>
              </button>
            ))}
          </div>
        )}
      </section>
      {error && <p className="notice bad">The catalog is unavailable.</p>}
      {isLoading ? (
        <Empty>Loading…</Empty>
      ) : list.length ? (
        <section className="rows" aria-label="Applications">
          {list.map((e) => (
            <CatalogRow key={e.id} entry={e} allowed={allowed} busy={busy} onRun={() => run(e.latest, e.title)} />
          ))}
        </section>
      ) : (
        <Empty>No applications are published yet.</Empty>
      )}
    </main>
  );
}

function CatalogRow({ entry, allowed, busy, onRun }: { entry: CatalogEntry; allowed: boolean; busy: boolean; onRun: () => void }) {
  const html = entry.latest.files.find((f) => f.role === "html");
  return (
    <article className="ri">
      <Link className="ri-thumb" to={"/apps/" + entry.id} tabIndex={-1}>
        <Monogram title={entry.title} caps={entry.latest.capabilities} />
      </Link>
      <div className="ri-text">
        <div className="ri-head">
          <h2 className="ri-title">
            <Link to={"/apps/" + entry.id}>{entry.title}</Link>
          </h2>
          <p className="ri-desc">{entry.description || "No description."}</p>
        </div>
        <div className="ri-foot">
          <p className="ri-tags">
            {entry.tags.join(" · ") || "untagged"}
            {entry.latest.downloadAccess === "free_sample" && " · free sample"}
          </p>
          <p className="ri-actions">
            <button className="tlink" onClick={onRun} disabled={busy}>
              Run
            </button>
            <Link className="tlink quiet" to={"/apps/" + entry.id}>
              Details
            </Link>
            {html && downloadable(entry.latest, allowed) && (
              <a className="tlink quiet" href={`/api/releases/${entry.latest.id}/files/${html.id}/download`}>
                Download
              </a>
            )}
            <span className="ri-path mono">
              v{entry.latest.version} · {html ? bytes(html.size) : ""} · {date(entry.latest.publishedAt)}
            </span>
          </p>
        </div>
      </div>
    </article>
  );
}

export function AppDetail({ id }: { id: string }) {
  const { data: session } = api.useSessionQuery();
  const { data: a, isLoading, error } = api.useArtifactQuery(id);
  const { run, busy } = useRun();
  if (isLoading) return <main><Empty>Loading…</Empty></main>;
  if (error || !a) return <main className="lost"><h1>Not found.</h1><Link className="u" to="/">Back to the catalog</Link></main>;
  const allowed = session?.access?.allowed === true;
  const latest = a.releases[0];
  return (
    <main>
      <section className="intro">
        <h1>
          {a.title} <span>— {a.description || "an HTML application"}</span>
        </h1>
        <p className="lead-actions">
          <button className="btn btn-ink" onClick={() => run(latest, a.title)} disabled={busy}>
            {session?.authenticated ? (allowed ? "Run in a new private instance" : "Subscribe to run") : "Sign in to run"}
          </button>
          <span className="muted">
            v{latest.version} · {latest.capabilities.join(" · ")}
          </span>
        </p>
      </section>
      <section className="lines">
        {a.releases.map((r) => {
          const html = r.files.find((f) => f.role === "html");
          const manifest = r.files.find((f) => f.role === "manifest");
          return (
            <div className="line release" key={r.id}>
              <div>
                <strong>v{r.version}</strong>
                <span className="muted">{date(r.publishedAt)}</span>
              </div>
              <div className="release-body">
                <p className="changelog">{r.changelog || "No release notes."}</p>
                <p className="release-meta mono muted">
                  {html && <>sha256 {html.sha256.slice(0, 16)}… · {bytes(html.size)}</>}
                </p>
                <p className="ri-actions">
                  <button className="tlink" onClick={() => run(r, a.title + " " + r.version)} disabled={busy}>
                    Run this version
                  </button>
                  {html && downloadable(r, allowed) && (
                    <a className="tlink quiet" href={`/api/releases/${r.id}/files/${html.id}/download`}>
                      Download HTML
                    </a>
                  )}
                  {manifest && downloadable(r, allowed) && (
                    <a className="tlink quiet" href={`/api/releases/${r.id}/files/${manifest.id}/download`}>
                      Manifest
                    </a>
                  )}
                </p>
              </div>
            </div>
          );
        })}
      </section>
      <section className="prose small">
        <h2>Run it locally</h2>
        <p>
          Subscribers may download a release and import it into their own Francois with their own Pi model configuration. Downloaded copies stay on your
          machine; hosted instances keep their data on this service.
        </p>
        <pre className="code-inline">go run ./cmd/francois import --id {a.id} --file ~/Downloads/{latest.files.find((f) => f.role === "html")?.name}</pre>
      </section>
    </main>
  );
}
