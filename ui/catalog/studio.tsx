import React, { Fragment, useEffect, useMemo, useRef, useState } from "react";
import { useDispatch } from "react-redux";
import { api, Check, errorMessage, Release } from "./api";
import { navigate, show } from "./store";
import { ArtifactRow, Creator, Media, Page, Post, studio } from "./studio-api";
import { bytes, date, Empty, Link, Row, useAct, useRoute } from "./ui";

// The studio is where creators and administrators manage storefront
// content: artifacts (details, page write-up, releases, demo), journal
// posts, creator profiles and the media library.

const sections: [string, string][] = [
  ["", "Artifacts"],
  ["journal", "Journal"],
  ["creators", "Creators"],
  ["media", "Media"],
  ["tokens", "Tokens"],
];

export function Studio() {
  const { data: session } = api.useSessionQuery();
  const { path } = useRoute();
  const parts = path.split("/").filter(Boolean); // ["studio", section, id]
  const section = parts[1] === "a" ? "" : (parts[1] ?? "");
  const id = parts[2];
  if (!session)
    return (
      <main>
        <Empty>Loading…</Empty>
      </main>
    );
  const role = session.user?.role;
  if (role !== "creator" && role !== "admin")
    return (
      <main className="lost">
        <h1>The studio is for creators.</h1>
        <Link className="u" to="/apps">
          Back to the catalog
        </Link>
      </main>
    );
  return (
    <main className="admin">
      <nav className="views admin-nav" aria-label="Studio">
        {sections.map(([s, label]) => (
          <Link
            key={s}
            to={"/studio" + (s ? "/" + s : "")}
            aria-current={section === s ? "page" : undefined}
          >
            {label}
          </Link>
        ))}
      </nav>
      <ProfileNotice />
      {section === "" && (id ? <ArtifactEditor id={id} /> : <Artifacts />)}
      {section === "journal" && (id ? <PostEditor id={id} /> : <Journal />)}
      {section === "creators" && <Creators />}
      {section === "media" && <MediaLibrary />}
      {section === "tokens" && <Tokens />}
    </main>
  );
}

function ProfileNotice() {
  const { data } = studio.useStudioMeQuery();
  if (!data || data.creator || data.user.role === "admin") return null;
  return (
    <p className="notice">
      Your account is not linked to a creator profile yet. Ask an administrator
      to link it before creating artifacts.
    </p>
  );
}

// Still shows a media item as a still: an image, a video's poster, or a
// video's first frame when it has no poster.
function Still({ id, className }: { id: string; className?: string }) {
  const { data = [] } = studio.useMediaQuery();
  const m = data.find((x) => x.id === id);
  if (m && m.mediaType.startsWith("video/")) {
    return m.poster ? (
      <img className={className} src={"/media/" + m.poster} alt="" />
    ) : (
      <video
        className={className}
        src={"/media/" + id}
        muted
        preload="metadata"
      />
    );
  }
  return <img className={className} src={"/media/" + id} alt="" />;
}

function Thumb({ media, title }: { media?: string; title: string }) {
  if (media) return <Still className="lib-thumb" id={media} />;
  return (
    <div className="lib-thumb mono-mini">{title.slice(0, 2).toUpperCase()}</div>
  );
}

function Artifacts() {
  const { data = [], isLoading } = studio.useStudioArtifactsQuery();
  const [create, { isLoading: creating }] = studio.useCreateArtifactMutation();
  const act = useAct();
  const [form, setForm] = useState({ id: "", title: "" });
  return (
    <>
      <section className="library">
        <div className="lib-head">
          <h2>Artifacts</h2>
          <form
            className="inline-form"
            onSubmit={async (e) => {
              e.preventDefault();
              const a = await act(create(form).unwrap(), "Artifact created");
              if (a) navigate("/studio/a/" + a.id);
            }}
          >
            <input
              placeholder="title"
              required
              value={form.title}
              onChange={(e) =>
                setForm({
                  title: e.target.value,
                  id: form.id || slugify(e.target.value),
                })
              }
            />
            <input
              className="slug"
              placeholder="id"
              required
              pattern="[a-z0-9][a-z0-9_\-]*"
              value={form.id}
              onChange={(e) => setForm({ ...form, id: e.target.value })}
            />
            <button className="btn btn-ink" disabled={creating}>
              New artifact
            </button>
          </form>
        </div>
        {isLoading ? (
          <Empty>Loading…</Empty>
        ) : data.length === 0 ? (
          <Empty>
            No artifacts yet. Create one, or upload a release for a new ID.
          </Empty>
        ) : (
          <div className="lib-list">
            {data.map((a: ArtifactRow) => (
              <Link
                className={
                  "lib-row" + (a.visibility === "hidden" ? " is-unlisted" : "")
                }
                key={a.id}
                to={"/studio/a/" + a.id}
              >
                <Thumb media={a.cover} title={a.title} />
                <span className="lib-main">
                  <strong>{a.title}</strong>
                  <span className="muted small">
                    {a.id}
                    {a.featured ? " · featured" : ""}
                    {a.visibility === "hidden" ? " · hidden" : ""}
                    {a.hasDraft ? " · unpublished page edits" : ""}
                  </span>
                </span>
                <span className="lib-status">
                  {a.latestVersion ? "v" + a.latestVersion : "unreleased"}
                </span>
                <span className="lib-added">{date(a.updatedAt)}</span>
              </Link>
            ))}
          </div>
        )}
      </section>
    </>
  );
}

export const slugify = (s: string) =>
  s
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "")
    .slice(0, 64);

const tabs = ["details", "page", "releases", "demo"] as const;
type Tab = (typeof tabs)[number];

function ArtifactEditor({ id }: { id: string }) {
  const { data, isLoading, error } = studio.useStudioArtifactQuery(id);
  const { search } = useRoute();
  const tab = (new URLSearchParams(search).get("tab") as Tab) || "details";
  if (isLoading) return <Empty>Loading…</Empty>;
  if (error || !data)
    return <Empty>{errorMessage(error, "Artifact not found")}</Empty>;
  const a = data.artifact;
  return (
    <section className="sheet">
      <div className="stats-head">
        <h1 className="h-display">{a.title}</h1>
        <span className="actions">
          <a
            className="u"
            href={"/apps/" + a.id}
            target="_blank"
            rel="noopener"
          >
            Public page
          </a>
          <a
            className="u"
            href={"/apps/" + a.id + "?draft=1"}
            target="_blank"
            rel="noopener"
          >
            Preview draft
          </a>
        </span>
      </div>
      <nav className="seg editor-tabs" aria-label="Editor">
        {tabs.map((t) => (
          <button
            key={t}
            aria-pressed={tab === t}
            onClick={() => navigate(`/studio/a/${a.id}?tab=${t}`, true)}
          >
            {
              {
                details: "Details",
                page: "Page",
                releases: "Releases",
                demo: "Demo",
              }[t]
            }
          </button>
        ))}
      </nav>
      {tab === "details" && <Details key={a.updatedAt} data={data} />}
      {tab === "page" && (
        <PageEditor
          key={a.id}
          id={a.id}
          initial={a.pageDraft ?? a.page}
          hasDraft={!!a.pageDraft}
        />
      )}
      {tab === "releases" && (
        <Releases artifact={a.id} releases={data.releases} />
      )}
      {tab === "demo" && <Demo id={a.id} enabled={a.demoEnabled} />}
    </section>
  );
}

function Details({
  data,
}: {
  data: { artifact: import("./studio-api").StudioArtifact; creators: string[] };
}) {
  const a = data.artifact;
  const { data: creators = [] } = studio.useCreatorsQuery();
  const [patch, { isLoading }] = studio.usePatchArtifactMutation();
  const act = useAct();
  const [f, setF] = useState({
    title: a.title,
    description: a.description,
    tags: a.tags.join(", "),
    visibility: a.visibility,
    featured: a.featured,
    sortOrder: a.sortOrder,
    earlyAccessUntil: a.earlyAccessUntil
      ? new Date(a.earlyAccessUntil).toISOString()
      : "",
    creators: data.creators,
  });
  const toggle = (c: string) =>
    setF({
      ...f,
      creators: f.creators.includes(c)
        ? f.creators.filter((x) => x !== c)
        : [...f.creators, c],
    });
  return (
    <form
      className="form editor-form"
      onSubmit={(e) => {
        e.preventDefault();
        void act(
          patch({
            id: a.id,
            ...f,
            tags: f.tags
              .split(",")
              .map((t) => t.trim())
              .filter(Boolean),
            sortOrder: Number(f.sortOrder),
          }).unwrap(),
          "Saved",
        );
      }}
    >
      <div className="row2">
        <div className="field">
          <label htmlFor="d-title">Title</label>
          <input
            id="d-title"
            required
            value={f.title}
            onChange={(e) => setF({ ...f, title: e.target.value })}
          />
        </div>
        <div className="field">
          <label htmlFor="d-vis">Visibility</label>
          <select
            id="d-vis"
            value={f.visibility}
            onChange={(e) =>
              setF({ ...f, visibility: e.target.value as "listed" | "hidden" })
            }
          >
            <option value="listed">Listed in the catalog</option>
            <option value="hidden">Hidden</option>
          </select>
        </div>
      </div>
      <label className="field">
        <span>Early access ends (UTC)</span>
        <input
          type="datetime-local"
          value={f.earlyAccessUntil.slice(0, 16)}
          onChange={(e) =>
            setF({
              ...f,
              earlyAccessUntil: e.target.value
                ? new Date(e.target.value + "Z").toISOString()
                : "",
            })
          }
        />
        <span className="muted small">
          Paying access is required to run or download before this date. Clear
          to end early access. Before first publication, a blank date uses the
          default publishing period.
        </span>
      </label>
      <div className="field">
        <label htmlFor="d-desc">Short description</label>
        <textarea
          id="d-desc"
          rows={2}
          maxLength={2000}
          value={f.description}
          onChange={(e) => setF({ ...f, description: e.target.value })}
        />
        <span className="muted small">
          Shown on cards and in link previews.
        </span>
      </div>
      <div className="row2">
        <div className="field">
          <label htmlFor="d-tags">Tags</label>
          <input
            id="d-tags"
            placeholder="tools, writing"
            value={f.tags}
            onChange={(e) => setF({ ...f, tags: e.target.value })}
          />
        </div>
      </div>
      <div className="row2">
        <label className="check">
          <input
            type="checkbox"
            checked={f.featured}
            onChange={(e) => setF({ ...f, featured: e.target.checked })}
          />{" "}
          Featured on the front page
        </label>
        <div className="field">
          <label htmlFor="d-sort">Sort order</label>
          <input
            id="d-sort"
            type="number"
            value={f.sortOrder}
            onChange={(e) => setF({ ...f, sortOrder: Number(e.target.value) })}
          />
        </div>
      </div>
      <div className="field">
        <span>Creators</span>
        <div className="seg">
          {creators.map((c: Creator) => (
            <button
              type="button"
              key={c.id}
              aria-pressed={f.creators.includes(c.id)}
              onClick={() => toggle(c.id)}
            >
              {c.name}
            </button>
          ))}
        </div>
      </div>
      <div className="form-foot">
        <span className="muted small">These settings apply immediately.</span>
        <button className="btn btn-ink" disabled={isLoading}>
          Save
        </button>
      </div>
    </form>
  );
}

// useMarkdownPreview renders Markdown with the server's public renderer,
// debounced, so the preview matches the published page exactly.
function useMarkdownPreview(source: string) {
  const [render] = studio.useMarkdownMutation();
  const [html, setHTML] = useState("");
  useEffect(() => {
    const t = setTimeout(() => {
      render(source)
        .unwrap()
        .then((r) => setHTML(r.html))
        .catch(() => undefined);
    }, 300);
    return () => clearTimeout(t);
  }, [source]);
  return html;
}

function MarkdownField({
  value,
  onChange,
  rows = 18,
  label,
}: {
  value: string;
  onChange: (v: string) => void;
  rows?: number;
  label: string;
}) {
  const html = useMarkdownPreview(value);
  return (
    <div className="md-editor">
      <div className="field">
        <label>{label}</label>
        <textarea
          className="slug"
          rows={rows}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          spellCheck
        />
      </div>
      {/* Rendered by the server's Markdown renderer, which emits no raw HTML. */}
      <div
        className="md md-preview"
        dangerouslySetInnerHTML={{ __html: html }}
      />
    </div>
  );
}

function PageEditor({
  id,
  initial,
  hasDraft,
}: {
  id: string;
  initial: Page;
  hasDraft: boolean;
}) {
  const [page, setPage] = useState<Page>({
    body: initial.body ?? "",
    cover: initial.cover,
    gallery: initial.gallery ?? [],
  });
  const [dirty, setDirty] = useState(false);
  const [save, { isLoading: saving }] = studio.useSaveDraftMutation();
  const [publish] = studio.usePublishDraftMutation();
  const [discard] = studio.useDiscardDraftMutation();
  const act = useAct();
  const [picking, setPicking] = useState<"" | "cover" | "gallery" | "insert">(
    "",
  );
  const set = (p: Partial<Page>) => {
    setPage({ ...page, ...p });
    setDirty(true);
  };
  const saveDraft = async () => {
    const ok = await act(save({ id, ...page }).unwrap(), "Draft saved");
    if (ok !== undefined) setDirty(false);
    return ok !== undefined;
  };
  const pick = (m: Media) => {
    if (picking === "cover") set({ cover: m.id });
    if (picking === "gallery") set({ gallery: [...page.gallery, m.id] });
    if (picking === "insert")
      set({ body: page.body + `\n\n![${m.alt}](/media/${m.id})\n` });
    setPicking("");
  };
  return (
    <div className="form editor-form">
      <p className="muted small">
        {dirty
          ? "Unsaved changes."
          : hasDraft
            ? "A draft is saved; visitors still see the published page."
            : "Showing the published page."}{" "}
        Save a draft, check it with <em>Preview draft</em>, then publish.
      </p>
      <MarkdownField
        label="Write-up (Markdown)"
        value={page.body}
        onChange={(body) => set({ body })}
      />
      <div className="row2">
        <div className="field">
          <span>Cover</span>
          <div className="media-strip">
            {page.cover && (
              <MediaThumb id={page.cover} onRemove={() => set({ cover: "" })} />
            )}
            <button
              type="button"
              className="btn btn-soft"
              onClick={() => setPicking("cover")}
            >
              {page.cover ? "Change cover" : "Choose cover"}
            </button>
          </div>
        </div>
        <div className="field">
          <span>Insert into the write-up</span>
          <div className="media-strip">
            <button
              type="button"
              className="btn btn-soft"
              onClick={() => setPicking("insert")}
            >
              Insert image
            </button>
          </div>
        </div>
      </div>
      <div className="field">
        <span>Gallery</span>
        <div className="media-strip">
          {page.gallery.map((g, i) => (
            <MediaThumb
              key={g + i}
              id={g}
              onRemove={() =>
                set({ gallery: page.gallery.filter((_, j) => j !== i) })
              }
            />
          ))}
          <button
            type="button"
            className="btn btn-soft"
            onClick={() => setPicking("gallery")}
          >
            Add to gallery
          </button>
        </div>
      </div>
      {picking && <MediaPicker onPick={pick} onClose={() => setPicking("")} />}
      <div className="form-foot sticky">
        <button
          type="button"
          className="btn btn-soft"
          disabled={!hasDraft}
          onClick={() =>
            confirm("Discard the saved draft?") &&
            act(discard(id).unwrap(), "Draft discarded")
          }
        >
          Discard draft
        </button>
        <span className="actions">
          <button
            type="button"
            className="btn btn-soft"
            disabled={saving}
            onClick={saveDraft}
          >
            Save draft
          </button>
          <button
            type="button"
            className="btn btn-ink"
            onClick={async () => {
              if (dirty && !(await saveDraft())) return;
              await act(publish(id).unwrap(), "Page published");
            }}
          >
            Publish page
          </button>
        </span>
      </div>
    </div>
  );
}

function MediaThumb({ id, onRemove }: { id: string; onRemove?: () => void }) {
  return (
    <span className="media-thumb">
      <Still id={id} />
      {onRemove && (
        <button type="button" className="tlink quiet" onClick={onRemove}>
          remove
        </button>
      )}
    </span>
  );
}

function MediaPicker({
  onPick,
  onClose,
  images,
}: {
  onPick: (m: Media) => void;
  onClose: () => void;
  images?: boolean;
}) {
  const { data: all = [] } = studio.useMediaQuery();
  const data = images ? all.filter((m) => !isVideo(m)) : all;
  return (
    <div className="picker">
      <div className="lib-head">
        <h2>Choose media</h2>
        <button type="button" className="tlink" onClick={onClose}>
          Close
        </button>
      </div>
      <Uploader />
      <div className="media-grid">
        {data.map((m) => (
          <button
            type="button"
            className="media-tile"
            key={m.id}
            onClick={() => onPick(m)}
          >
            <MediaPreview m={m} />
            <span className="muted small">{m.alt || m.id}</span>
          </button>
        ))}
      </div>
    </div>
  );
}

const isVideo = (m: Media) => m.mediaType.startsWith("video/");

function MediaPreview({ m }: { m: Media }) {
  return isVideo(m) ? (
    <video
      src={"/media/" + m.id}
      poster={m.poster ? "/media/" + m.poster : undefined}
      muted
      preload="metadata"
    />
  ) : (
    <img src={"/media/" + m.id} alt={m.alt} loading="lazy" />
  );
}

// captureFrame draws the video's current frame to a canvas. Media is
// same-origin, so the canvas is not tainted and can be exported.
function captureFrame(v: HTMLVideoElement): Promise<Blob> {
  const c = document.createElement("canvas");
  c.width = v.videoWidth;
  c.height = v.videoHeight;
  c.getContext("2d")?.drawImage(v, 0, 0);
  return new Promise((resolve, reject) =>
    c.toBlob(
      (b) =>
        b ? resolve(b) : reject(new Error("Could not capture the frame")),
      "image/jpeg",
      0.9,
    ),
  );
}

// VideoTile lets the editor scrub to a frame and use it as the poster, or
// pick an image from the library.
function VideoTile({ m, editable }: { m: Media; editable: boolean }) {
  const video = useRef<HTMLVideoElement>(null);
  const [upload] = studio.useUploadMediaMutation();
  const [patch] = studio.usePatchMediaMutation();
  const [picking, setPicking] = useState(false);
  const act = useAct();
  const frameAsPoster = async () => {
    const v = video.current;
    if (!v || !v.videoWidth) return;
    const blob = await captureFrame(v);
    const fd = new FormData();
    fd.set("file", new File([blob], "poster.jpg", { type: "image/jpeg" }));
    fd.set("alt", ("Poster: " + m.alt).slice(0, 300));
    const poster = await upload(fd).unwrap();
    await patch({ id: m.id, poster: poster.id }).unwrap();
  };
  return (
    <>
      <video
        ref={video}
        src={"/media/" + m.id}
        poster={m.poster ? "/media/" + m.poster : undefined}
        controls
        muted
        preload="metadata"
      />
      {editable && (
        <span className="actions">
          <button
            type="button"
            className="tlink"
            onClick={() => act(frameAsPoster(), "Poster set from this frame")}
          >
            Use this frame as poster
          </button>
          <button
            type="button"
            className="tlink"
            onClick={() => setPicking(true)}
          >
            Choose poster
          </button>
          {m.poster && (
            <button
              type="button"
              className="tlink quiet"
              onClick={() =>
                act(patch({ id: m.id, poster: "" }).unwrap(), "Poster removed")
              }
            >
              No poster
            </button>
          )}
        </span>
      )}
      {picking && (
        <MediaPicker
          images
          onClose={() => setPicking(false)}
          onPick={(p) => {
            setPicking(false);
            void act(patch({ id: m.id, poster: p.id }).unwrap(), "Poster set");
          }}
        />
      )}
    </>
  );
}

function Uploader() {
  const [upload, { isLoading }] = studio.useUploadMediaMutation();
  const { data: me } = studio.useStudioMeQuery();
  const act = useAct();
  const input = useRef<HTMLInputElement>(null);
  const [over, setOver] = useState(false);
  const send = async (files?: FileList | null) => {
    for (const f of Array.from(files ?? [])) {
      const fd = new FormData();
      fd.set("file", f);
      fd.set("alt", f.name.replace(/\.[a-z0-9]+$/i, "").replace(/[-_]+/g, " "));
      await act(upload(fd).unwrap(), "Uploaded " + f.name);
    }
  };
  return (
    <div
      className={"drop drop-small" + (over ? " is-over" : "")}
      role="button"
      tabIndex={0}
      onClick={() => input.current?.click()}
      onKeyDown={(e) =>
        (e.key === "Enter" || e.key === " ") && input.current?.click()
      }
      onDragOver={(e) => {
        e.preventDefault();
        setOver(true);
      }}
      onDragLeave={() => setOver(false)}
      onDrop={(e) => {
        e.preventDefault();
        setOver(false);
        void send(e.dataTransfer.files);
      }}
    >
      <span>
        {isLoading ? "Uploading…" : "Drop images or videos, or click to choose"}
      </span>
      <span className="muted small">
        PNG, JPEG, GIF, WebP, MP4, WebM
        {me ? " · up to " + bytes(me.limits.maxMediaBytes) : ""}
      </span>
      <input
        ref={input}
        type="file"
        multiple
        accept="image/png,image/jpeg,image/gif,image/webp,video/mp4,video/webm"
        hidden
        onChange={(e) => send(e.target.files)}
      />
    </div>
  );
}

function MediaLibrary() {
  const { data: session } = api.useSessionQuery();
  const { data = [], isLoading } = studio.useMediaQuery();
  const mine = (m: Media) =>
    session?.user?.role === "admin" || m.uploadedBy === session?.user?.id;
  const [patch] = studio.usePatchMediaMutation();
  const [del] = studio.useDeleteMediaMutation();
  const act = useAct();
  const dispatch = useDispatch();
  return (
    <section className="library">
      <div className="lib-head">
        <h2>Media</h2>
      </div>
      <Uploader />
      {isLoading ? (
        <Empty>Loading…</Empty>
      ) : data.length === 0 ? (
        <Empty>No media yet.</Empty>
      ) : (
        <div className="media-grid">
          {data.map((m) => (
            <div className="media-tile" key={m.id}>
              {isVideo(m) ? (
                <VideoTile m={m} editable={mine(m)} />
              ) : (
                <MediaPreview m={m} />
              )}
              <input
                aria-label="Alt text"
                disabled={!mine(m)}
                defaultValue={m.alt}
                placeholder="alt text"
                onBlur={(e) =>
                  e.target.value !== m.alt &&
                  act(
                    patch({ id: m.id, alt: e.target.value }).unwrap(),
                    "Alt text saved",
                  )
                }
              />
              <span className="muted small">
                {m.width ? `${m.width}×${m.height} · ` : ""}
                {bytes(m.size)} · {date(m.createdAt)}
              </span>
              <span className="actions">
                <button
                  type="button"
                  className="tlink"
                  onClick={() => {
                    void navigator.clipboard.writeText(
                      `![${m.alt}](/media/${m.id})`,
                    );
                    dispatch(show({ text: "Markdown copied" }));
                  }}
                >
                  Copy Markdown
                </button>
                {mine(m) && (
                  <button
                    type="button"
                    className="tlink quiet danger"
                    onClick={() =>
                      confirm("Delete this media?") &&
                      act(del(m.id).unwrap(), "Deleted")
                    }
                  >
                    Delete
                  </button>
                )}
              </span>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

const defaultManifest =
  '{\n  "sample": {},\n  "db": {},\n  "user": {"scopes": ["profile"]},\n  "room": {}\n}';

function Releases({
  artifact,
  releases,
}: {
  artifact: string;
  releases: Release[];
}) {
  const [check, { isLoading: checking }] = studio.useCheckReleaseMutation();
  const [upload, { isLoading: uploading }] = studio.useUploadReleaseMutation();
  const [action] = studio.useReleaseActionMutation();
  const [preview] = studio.usePreviewReleaseMutation();
  const act = useAct();
  const dispatch = useDispatch();
  const [file, setFile] = useState<File>();
  const [over, setOver] = useState(false);
  const [form, setForm] = useState({
    version: "",
    changelog: "",
    manifest: "",
    freeSample: false,
  });
  const [checks, setChecks] = useState<Check[] | null>(null);
  const [open, setOpen] = useState("");
  const input = useRef<HTMLInputElement>(null);
  const latestManifest = useMemo(
    () => releases.find((r) => r.manifest)?.manifest,
    [releases],
  );
  const manifest =
    form.manifest ||
    (latestManifest
      ? JSON.stringify(latestManifest, null, 2)
      : defaultManifest);
  const set = (k: keyof typeof form, v: string | boolean) => {
    setForm({ ...form, [k]: v });
    setChecks(null);
  };
  const formData = () => {
    const fd = new FormData();
    if (file) fd.set("html", file);
    fd.set("artifactId", artifact);
    fd.set("version", form.version);
    fd.set("changelog", form.changelog);
    fd.set("manifestJson", manifest);
    fd.set("freeSample", String(form.freeSample));
    return fd;
  };
  const save = async () => {
    try {
      const rel = await upload(formData()).unwrap();
      setChecks(rel.checks ?? []);
      dispatch(
        show({
          text: `Draft v${rel.version} saved. Preview it, then publish.`,
        }),
      );
      setFile(undefined);
      setForm({ version: "", changelog: "", manifest: "", freeSample: false });
    } catch (e: any) {
      if (e?.data?.checks) setChecks(e.data.checks);
      dispatch(show({ text: errorMessage(e), bad: true }));
    }
  };
  return (
    <>
      <section className="publish">
        <div className="publish-left">
          <div
            className={"drop" + (over ? " is-over" : "")}
            role="button"
            tabIndex={0}
            onClick={() => input.current?.click()}
            onKeyDown={(e) =>
              (e.key === "Enter" || e.key === " ") && input.current?.click()
            }
            onDragOver={(e) => {
              e.preventDefault();
              setOver(true);
            }}
            onDragLeave={() => setOver(false)}
            onDrop={(e) => {
              e.preventDefault();
              setOver(false);
              setFile(e.dataTransfer.files[0]);
              setChecks(null);
            }}
          >
            <span className="drop-icon">↥</span>
            <span>
              {file ? file.name : "Drop an HTML file, or click to choose"}
            </span>
            <span className="muted small">
              {file
                ? bytes(file.size)
                : "single-file application, authored source"}
            </span>
            <input
              ref={input}
              type="file"
              accept=".html,.htm,text/html"
              hidden
              onChange={(e) => (setFile(e.target.files?.[0]), setChecks(null))}
            />
          </div>
          <p className="muted small">
            Uploading creates a <em>draft</em>. Nobody sees it until you preview
            it on an isolated origin and publish. Published files are immutable;
            corrections are new versions.
          </p>
          {checks && (
            <div className="queue">
              {checks.length === 0 && (
                <div className="qrow">
                  <span className="qname">No findings.</span>
                </div>
              )}
              {checks.map((c, i) => (
                <div className="qrow" key={i}>
                  <span
                    className={"qstatus " + (c.level === "error" ? "warn" : "")}
                  >
                    {c.level}
                  </span>
                  <span className="qname">{c.message}</span>
                </div>
              ))}
            </div>
          )}
        </div>
        <form
          className="form"
          onSubmit={(e) => {
            e.preventDefault();
            void save();
          }}
        >
          <div className="field">
            <label htmlFor="r-version">Version</label>
            <input
              id="r-version"
              className="slug"
              required
              placeholder="1.0.0"
              value={form.version}
              onChange={(e) => set("version", e.target.value)}
            />
          </div>
          <div className="field">
            <label htmlFor="r-notes">Release notes</label>
            <textarea
              id="r-notes"
              rows={3}
              value={form.changelog}
              onChange={(e) => set("changelog", e.target.value)}
            />
          </div>
          <div className="field">
            <label htmlFor="r-manifest">Capabilities manifest</label>
            <textarea
              id="r-manifest"
              className="slug"
              rows={7}
              spellCheck={false}
              value={manifest}
              onChange={(e) => set("manifest", e.target.value)}
            />
          </div>
          <label className="check">
            <input
              type="checkbox"
              checked={form.freeSample}
              onChange={(e) => set("freeSample", e.target.checked)}
            />{" "}
            Free sample (downloadable without a subscription)
          </label>
          <div className="form-foot">
            <button
              type="button"
              className="btn btn-soft"
              disabled={!file || checking}
              onClick={async () =>
                setChecks(
                  (await act(check(formData()).unwrap()))?.checks ?? null,
                )
              }
            >
              {checking ? "Checking…" : "Check"}
            </button>
            <button className="btn btn-ink" disabled={!file || uploading}>
              {uploading ? "Saving…" : "Save draft"}
            </button>
          </div>
        </form>
      </section>
      <section className="library">
        <div className="lib-head">
          <h2>Versions</h2>
        </div>
        {releases.length === 0 ? (
          <Empty>No releases yet.</Empty>
        ) : (
          <div className="lib-list">
            {releases.map((r) => {
              const html = r.files.find((f) => f.role === "html");
              return (
                <Fragment key={r.id}>
                  <div
                    className={
                      "lib-row" +
                      (r.state === "withdrawn" ? " is-unlisted" : "")
                    }
                  >
                    <div className="lib-main">
                      <strong>v{r.version}</strong>
                      <span className="muted small">
                        {r.capabilities.join(" · ")}
                        {r.freeSample ? " · free sample" : ""}
                      </span>
                    </div>
                    <span className="lib-size">
                      {html ? bytes(html.size) : ""}
                    </span>
                    <span className="lib-added">
                      {date(r.publishedAt ?? r.createdAt)}
                    </span>
                    <span
                      className={
                        "lib-status" +
                        (r.state === "published"
                          ? " on"
                          : r.state === "withdrawn"
                            ? " bad"
                            : "")
                      }
                    >
                      {r.state}
                      {r.pinnedInstances
                        ? ` · ${r.pinnedInstances} pinned`
                        : ""}
                    </span>
                    <div className="lib-actions">
                      <button
                        className="btn btn-soft"
                        onClick={async () => {
                          const inst = await act(preview(r.id).unwrap());
                          if (inst) navigate("/instances/" + inst.id);
                        }}
                      >
                        Preview
                      </button>
                      {r.state === "draft" && (
                        <>
                          <button
                            className="btn btn-ink"
                            onClick={() =>
                              act(
                                action({
                                  artifact,
                                  id: r.id,
                                  action: "publish",
                                }).unwrap(),
                                `Published v${r.version}`,
                              )
                            }
                          >
                            Publish
                          </button>
                          <button
                            className="btn btn-soft"
                            onClick={() =>
                              confirm("Delete this draft?") &&
                              act(
                                action({
                                  artifact,
                                  id: r.id,
                                  action: "delete",
                                }).unwrap(),
                                "Draft deleted",
                              )
                            }
                          >
                            Delete
                          </button>
                        </>
                      )}
                      {r.state === "published" && (
                        <button
                          className="btn btn-soft"
                          onClick={() => {
                            const n = r.pinnedInstances ?? 0;
                            if (
                              !confirm(
                                `Withdraw v${r.version} from the catalog?`,
                              )
                            )
                              return;
                            const suspend =
                              n > 0 &&
                              confirm(
                                `${n} instance(s) are pinned to this release. Suspend them too? (Cancel keeps them running.)`,
                              );
                            void act(
                              action({
                                artifact,
                                id: r.id,
                                action: "withdraw",
                                suspendPinned: suspend,
                              }).unwrap(),
                              "Release withdrawn",
                            );
                          }}
                        >
                          Withdraw
                        </button>
                      )}
                      <button
                        className="btn btn-soft"
                        aria-expanded={open === r.id}
                        onClick={() => setOpen(open === r.id ? "" : r.id)}
                      >
                        Attachments ({r.attachments.length})
                      </button>
                    </div>
                  </div>
                  {open === r.id && (
                    <Attachments artifact={artifact} release={r} />
                  )}
                </Fragment>
              );
            })}
          </div>
        )}
      </section>
    </>
  );
}

// Attachments are supplementary files on a release. Unlike the HTML they
// can change after publishing; visitors download them under the release's
// download rule.
function Attachments({
  artifact,
  release,
}: {
  artifact: string;
  release: Release;
}) {
  const [upload, { isLoading }] = studio.useUploadAttachmentMutation();
  const [patch] = studio.usePatchAttachmentMutation();
  const [del] = studio.useDeleteAttachmentMutation();
  const { data: me } = studio.useStudioMeQuery();
  const act = useAct();
  const input = useRef<HTMLInputElement>(null);
  const [over, setOver] = useState(false);
  const send = async (files?: FileList | null) => {
    for (const f of Array.from(files ?? [])) {
      const form = new FormData();
      form.set("file", f);
      await act(
        upload({ artifact, release: release.id, form }).unwrap(),
        "Attached " + f.name,
      );
    }
  };
  const list = release.attachments;
  // Reordering renumbers the list 0…n-1 and saves the positions that change.
  const move = (i: number, d: number) => {
    const order = [...list];
    [order[i], order[i + d]] = [order[i + d], order[i]];
    void act(
      Promise.all(
        order.flatMap((a, pos) =>
          a.position === pos
            ? []
            : [patch({ artifact, id: a.id, position: pos }).unwrap()],
        ),
      ),
    );
  };
  return (
    <div className="attach-panel">
      <div
        className={"drop drop-small" + (over ? " is-over" : "")}
        role="button"
        tabIndex={0}
        onClick={() => input.current?.click()}
        onKeyDown={(e) =>
          (e.key === "Enter" || e.key === " ") && input.current?.click()
        }
        onDragOver={(e) => {
          e.preventDefault();
          setOver(true);
        }}
        onDragLeave={() => setOver(false)}
        onDrop={(e) => {
          e.preventDefault();
          setOver(false);
          void send(e.dataTransfer.files);
        }}
      >
        <span>
          {isLoading
            ? "Uploading…"
            : `Drop files to attach to v${release.version}, or click to choose`}
        </span>
        <span className="muted small">
          Manuals, archives, notes · up to 20 files
          {me ? ", " + bytes(me.limits.maxAttachmentBytes) + " each" : ""} ·
          downloadable by whoever may download this release
        </span>
        <input
          ref={input}
          type="file"
          multiple
          hidden
          onChange={(e) => send(e.target.files)}
        />
      </div>
      {list.length === 0 ? (
        <Empty>No attachments.</Empty>
      ) : (
        <div className="lib-list">
          {list.map((a, i) => (
            <div className="lib-row" key={a.id}>
              <div className="lib-main">
                <input
                  aria-label={"Label for " + a.name}
                  defaultValue={a.label}
                  placeholder={a.name}
                  onBlur={(e) =>
                    e.target.value !== a.label &&
                    act(
                      patch({
                        artifact,
                        id: a.id,
                        label: e.target.value,
                      }).unwrap(),
                      "Label saved",
                    )
                  }
                />
                <span className="muted small">
                  {a.name} · {a.mediaType}
                </span>
              </div>
              <span className="lib-size">{bytes(a.size)}</span>
              <div className="lib-actions">
                <button
                  className="tlink"
                  disabled={i === 0}
                  aria-label="Move up"
                  onClick={() => move(i, -1)}
                >
                  ↑
                </button>
                <button
                  className="tlink"
                  disabled={i === list.length - 1}
                  aria-label="Move down"
                  onClick={() => move(i, 1)}
                >
                  ↓
                </button>
                <a className="tlink" href={a.href} download={a.name}>
                  Download
                </a>
                <button
                  className="tlink quiet danger"
                  onClick={() =>
                    confirm(`Remove ${a.name}?`) &&
                    act(del({ artifact, id: a.id }).unwrap(), "Removed")
                  }
                >
                  Remove
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function Demo({ id, enabled }: { id: string; enabled: boolean }) {
  const [patch] = studio.usePatchArtifactMutation();
  const act = useAct();
  return (
    <div className="form editor-form">
      <Row label="Visitor demo" sub="no account needed">
        <label className="check">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) =>
              act(
                patch({ id, demoEnabled: e.target.checked }).unwrap(),
                "Saved",
              )
            }
          />{" "}
          Offer a demo of the latest published release
        </label>
        <p className="muted">
          Visitors get a short-lived private instance with a small model budget.
          Demos also need to be enabled globally by an administrator in
          Settings.
        </p>
      </Row>
    </div>
  );
}

function Journal() {
  const { data = [], isLoading } = studio.usePostsQuery();
  const [create] = studio.useCreatePostMutation();
  const act = useAct();
  const [title, setTitle] = useState("");
  return (
    <section className="library">
      <div className="lib-head">
        <h2>Journal</h2>
        <form
          className="inline-form"
          onSubmit={async (e) => {
            e.preventDefault();
            const p = await act(
              create({ title, slug: slugify(title) }).unwrap(),
              "Post created",
            );
            if (p) navigate("/studio/journal/" + p.id);
          }}
        >
          <input
            placeholder="title of a new post"
            required
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
          <button className="btn btn-ink">New post</button>
        </form>
      </div>
      {isLoading ? (
        <Empty>Loading…</Empty>
      ) : data.length === 0 ? (
        <Empty>No posts yet.</Empty>
      ) : (
        <div className="lib-list">
          {data.map((p: Post) => (
            <Link className="lib-row" key={p.id} to={"/studio/journal/" + p.id}>
              <Thumb media={p.cover} title={p.title} />
              <span className="lib-main">
                <strong>{p.title}</strong>
                <span className="muted small">/journal/{p.slug}</span>
              </span>
              <span
                className={
                  "lib-status" + (p.state === "published" ? " on" : "")
                }
              >
                {p.state}
              </span>
              <span className="lib-added">
                {date(p.publishedAt ?? p.updatedAt)}
              </span>
            </Link>
          ))}
        </div>
      )}
    </section>
  );
}

function PostEditor({ id }: { id: string }) {
  const { data: post, isLoading } = studio.usePostQuery(id);
  if (isLoading || !post) return <Empty>Loading…</Empty>;
  return <PostForm key={post.updatedAt} post={post} />;
}

function PostForm({ post }: { post: Post }) {
  const { data: session } = api.useSessionQuery();
  const { data: creators = [] } = studio.useCreatorsQuery();
  const [patch, { isLoading }] = studio.usePatchPostMutation();
  const [action] = studio.usePostActionMutation();
  const act = useAct();
  const [f, setF] = useState({
    title: post.title,
    slug: post.slug,
    summary: post.summary,
    body: post.body,
    cover: post.cover ?? "",
    tags: post.tags.join(", "),
    authorId: post.authorId ?? "",
  });
  const [picking, setPicking] = useState<"" | "cover" | "insert">("");
  const isAdmin = session?.user?.role === "admin";
  const save = () =>
    act(
      patch({
        id: post.id,
        ...f,
        tags: f.tags
          .split(",")
          .map((t) => t.trim())
          .filter(Boolean),
        ...(isAdmin ? {} : { authorId: undefined }),
      }).unwrap(),
      "Saved",
    );
  return (
    <section className="sheet">
      <div className="stats-head">
        <h1 className="h-display">{post.title}</h1>
        <span className="actions">
          <a
            className="u"
            href={"/journal/" + post.slug}
            target="_blank"
            rel="noopener"
          >
            {post.state === "published" ? "Public page" : "Preview"}
          </a>
          <span
            className={"status" + (post.state === "published" ? " on" : "")}
          >
            {post.state}
          </span>
        </span>
      </div>
      <form
        className="form editor-form"
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
      >
        <div className="row2">
          <div className="field">
            <label htmlFor="p-title">Title</label>
            <input
              id="p-title"
              required
              value={f.title}
              onChange={(e) => setF({ ...f, title: e.target.value })}
            />
          </div>
          <div className="field">
            <label htmlFor="p-slug">Slug</label>
            <input
              id="p-slug"
              className="slug"
              required
              value={f.slug}
              onChange={(e) => setF({ ...f, slug: e.target.value })}
            />
          </div>
        </div>
        <div className="field">
          <label htmlFor="p-summary">Summary</label>
          <input
            id="p-summary"
            maxLength={400}
            value={f.summary}
            onChange={(e) => setF({ ...f, summary: e.target.value })}
          />
        </div>
        <div className="row2">
          <div className="field">
            <label htmlFor="p-tags">Tags</label>
            <input
              id="p-tags"
              value={f.tags}
              onChange={(e) => setF({ ...f, tags: e.target.value })}
            />
          </div>
          <div className="field">
            <label htmlFor="p-author">Author</label>
            <select
              id="p-author"
              disabled={!isAdmin}
              value={f.authorId}
              onChange={(e) => setF({ ...f, authorId: e.target.value })}
            >
              <option value="">—</option>
              {creators.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </select>
          </div>
        </div>
        <MarkdownField
          label="Body (Markdown)"
          value={f.body}
          onChange={(body) => setF({ ...f, body })}
        />
        <div className="field">
          <span>Cover</span>
          <div className="media-strip">
            {f.cover && (
              <MediaThumb
                id={f.cover}
                onRemove={() => setF({ ...f, cover: "" })}
              />
            )}
            <button
              type="button"
              className="btn btn-soft"
              onClick={() => setPicking("cover")}
            >
              {f.cover ? "Change cover" : "Choose cover"}
            </button>
            <button
              type="button"
              className="btn btn-soft"
              onClick={() => setPicking("insert")}
            >
              Insert image
            </button>
          </div>
        </div>
        {picking && (
          <MediaPicker
            onClose={() => setPicking("")}
            onPick={(m) => {
              if (picking === "cover") setF({ ...f, cover: m.id });
              else
                setF({
                  ...f,
                  body: f.body + `\n\n![${m.alt}](/media/${m.id})\n`,
                });
              setPicking("");
            }}
          />
        )}
        <div className="form-foot sticky">
          <button
            type="button"
            className="btn btn-soft"
            onClick={async () => {
              if (!confirm("Delete this post?")) return;
              if (
                (await act(
                  action({ id: post.id, action: "delete" }).unwrap(),
                  "Post deleted",
                )) !== undefined
              )
                navigate("/studio/journal");
            }}
          >
            Delete
          </button>
          <span className="actions">
            <button className="btn btn-soft" disabled={isLoading}>
              Save
            </button>
            <button
              type="button"
              className="btn btn-ink"
              onClick={async () => {
                if ((await save()) === undefined) return;
                await act(
                  action({
                    id: post.id,
                    action:
                      post.state === "published" ? "unpublish" : "publish",
                  }).unwrap(),
                  post.state === "published" ? "Unpublished" : "Published",
                );
              }}
            >
              {post.state === "published"
                ? "Save and unpublish"
                : "Save and publish"}
            </button>
          </span>
        </div>
      </form>
    </section>
  );
}

function Creators() {
  const { data: session } = api.useSessionQuery();
  const { data = [], isLoading } = studio.useCreatorsQuery();
  const [create] = studio.useCreateCreatorMutation();
  const act = useAct();
  const isAdmin = session?.user?.role === "admin";
  const [name, setName] = useState("");
  const [editing, setEditing] = useState<string>("");
  const editable = (c: Creator) => isAdmin || c.userId === session?.user?.id;
  return (
    <section className="library">
      <div className="lib-head">
        <h2>Creators</h2>
        {isAdmin && (
          <form
            className="inline-form"
            onSubmit={async (e) => {
              e.preventDefault();
              const c = await act(
                create({ name, slug: slugify(name) }).unwrap(),
                "Profile created",
              );
              if (c) {
                setName("");
                setEditing(c.id);
              }
            }}
          >
            <input
              placeholder="name"
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
            <button className="btn btn-ink">New profile</button>
          </form>
        )}
      </div>
      {isLoading ? (
        <Empty>Loading…</Empty>
      ) : (
        <div className="lib-list">
          {data.map((c) => (
            <React.Fragment key={c.id}>
              <div className="lib-row">
                <Thumb media={c.avatar} title={c.name} />
                <span className="lib-main">
                  <strong>{c.name}</strong>
                  <span className="muted small">
                    /creators/{c.slug}
                    {c.userId ? " · linked to an account" : " · no account"}
                  </span>
                </span>
                <div className="lib-actions">
                  <a
                    className="btn btn-soft"
                    href={"/creators/" + c.slug}
                    target="_blank"
                    rel="noopener"
                  >
                    View
                  </a>
                  {editable(c) && (
                    <button
                      className="btn btn-soft"
                      onClick={() => setEditing(editing === c.id ? "" : c.id)}
                    >
                      {editing === c.id ? "Close" : "Edit"}
                    </button>
                  )}
                </div>
              </div>
              {editing === c.id && (
                <CreatorForm
                  key={c.id}
                  creator={c}
                  isAdmin={isAdmin}
                  onDone={() => setEditing("")}
                />
              )}
            </React.Fragment>
          ))}
        </div>
      )}
    </section>
  );
}

function CreatorForm({
  creator,
  isAdmin,
  onDone,
}: {
  creator: Creator;
  isAdmin: boolean;
  onDone: () => void;
}) {
  const [patch, { isLoading }] = studio.usePatchCreatorMutation();
  const [del] = studio.useDeleteCreatorMutation();
  const act = useAct();
  const [f, setF] = useState({
    name: creator.name,
    slug: creator.slug,
    bio: creator.bio,
    avatar: creator.avatar ?? "",
    userId: creator.userId ?? "",
    sortOrder: creator.sortOrder,
  });
  const [links, setLinks] = useState(
    creator.links.map((l) => `${l.label} ${l.url}`).join("\n"),
  );
  const [picking, setPicking] = useState(false);
  return (
    <form
      className="form editor-form creator-form"
      onSubmit={async (e) => {
        e.preventDefault();
        const parsed = links
          .split("\n")
          .map((l) => l.trim())
          .filter(Boolean)
          .map((l) => {
            const i = l.lastIndexOf(" ");
            return { label: l.slice(0, i).trim(), url: l.slice(i + 1) };
          });
        const body = {
          id: creator.id,
          name: f.name,
          slug: f.slug,
          bio: f.bio,
          avatar: f.avatar,
          links: parsed,
          ...(isAdmin
            ? { userId: f.userId, sortOrder: Number(f.sortOrder) }
            : {}),
        };
        if ((await act(patch(body).unwrap(), "Profile saved")) !== undefined)
          onDone();
      }}
    >
      <div className="row2">
        <div className="field">
          <label>Name</label>
          <input
            required
            value={f.name}
            onChange={(e) => setF({ ...f, name: e.target.value })}
          />
        </div>
        <div className="field">
          <label>Slug</label>
          <input
            className="slug"
            required
            value={f.slug}
            onChange={(e) => setF({ ...f, slug: e.target.value })}
          />
        </div>
      </div>
      <MarkdownField
        label="Bio (Markdown)"
        rows={6}
        value={f.bio}
        onChange={(bio) => setF({ ...f, bio })}
      />
      <div className="field">
        <label>Links, one per line: label then URL</label>
        <textarea
          className="slug"
          rows={3}
          placeholder="site https://example.com"
          value={links}
          onChange={(e) => setLinks(e.target.value)}
        />
      </div>
      <div className="field">
        <span>Avatar</span>
        <div className="media-strip">
          {f.avatar && (
            <MediaThumb
              id={f.avatar}
              onRemove={() => setF({ ...f, avatar: "" })}
            />
          )}
          <button
            type="button"
            className="btn btn-soft"
            onClick={() => setPicking(true)}
          >
            Choose avatar
          </button>
        </div>
      </div>
      {picking && (
        <MediaPicker
          onClose={() => setPicking(false)}
          onPick={(m) => {
            setF({ ...f, avatar: m.id });
            setPicking(false);
          }}
        />
      )}
      {isAdmin && (
        <div className="row2">
          <div className="field">
            <label>Linked account (user ID)</label>
            <input
              className="slug"
              placeholder="usr_…"
              value={f.userId}
              onChange={(e) => setF({ ...f, userId: e.target.value })}
            />
            <span className="muted small">
              The linked account may edit this profile and every artifact
              credited to it.
            </span>
          </div>
          <div className="field">
            <label>Sort order</label>
            <input
              type="number"
              value={f.sortOrder}
              onChange={(e) =>
                setF({ ...f, sortOrder: Number(e.target.value) })
              }
            />
          </div>
        </div>
      )}
      <div className="form-foot">
        {isAdmin ? (
          <button
            type="button"
            className="btn btn-soft"
            onClick={() =>
              confirm("Delete this profile? Credits are removed.") &&
              act(del(creator.id).unwrap(), "Profile deleted")
            }
          >
            Delete profile
          </button>
        ) : (
          <span />
        )}
        <button className="btn btn-ink" disabled={isLoading}>
          Save profile
        </button>
      </div>
    </form>
  );
}

// Tokens moved to the account page, where every signed-in user can create
// them with scopes; the studio keeps a pointer for creators who look here.
function Tokens() {
  return (
    <section className="library">
      <div className="lib-head">
        <h2>API tokens</h2>
      </div>
      <p>
        API tokens are managed on your{" "}
        <Link className="u" to="/account#api-tokens">
          account page
        </Link>
        . Give a token the <span className="mono">studio</span> scope to use it
        with <span className="mono">francois catalog releases push</span>.
      </p>
    </section>
  );
}
