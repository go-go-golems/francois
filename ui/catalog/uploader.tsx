import React, { useEffect, useRef, useState } from "react";
import { errorMessage } from "./api";
import { studio, type Limits, type StudioDetail } from "./studio-api";
import { Link, bytes, useRoute } from "./ui";
import { MarkdownField, MediaPicker } from "./studio-fields";
import {
  capabilities,
  capabilityOn,
  editableCapability,
  initialValues,
  mediaAccept,
  parseManifest,
  slugify,
  toggleCapability,
  validateUpload,
  type MediaChoice,
  type UploadValues,
} from "./uploader-model";
import { submitUpload, type Stage, type UploadResult } from "./uploader-submit";

export function SlopUploader() {
  const { search } = useRoute();
  const [selected, setSelected] = useState(
    new URLSearchParams(search).get("artifact") ?? "",
  );
  const [revision, setRevision] = useState(0);
  const [busy, setBusy] = useState(false);
  const [dirty, setDirty] = useState(false);
  const { data: me, error } = studio.useStudioMeQuery();
  const { data: artifacts = [] } = studio.useStudioArtifactsQuery();
  const { currentData: detail, error: detailError } =
    studio.useStudioArtifactQuery(selected, { skip: !selected });
  function select(id: string) {
    if (busy || (dirty && !window.confirm("Discard the unsaved upload form?")))
      return;
    setSelected(id);
    setRevision((n) => n + 1);
    setDirty(false);
  }
  return (
    <div className="slop-uploader">
      <header>
        <p className="eyebrow">SLOP.BOUTIQUE / STUDIO</p>
        <h1>Upload something good.</h1>
        <p className="muted">
          One HTML file. A few details. Ready for the boutique.
        </p>
      </header>
      <label className="upload-selector">
        Artifact
        <select
          value={selected}
          disabled={busy}
          onChange={(e) => select(e.target.value)}
        >
          <option value="">New artifact</option>
          {artifacts.map((a) => (
            <option key={a.id} value={a.id}>
              {a.title}
            </option>
          ))}
        </select>
      </label>
      {error ? (
        <p role="alert">{errorMessage(error)}</p>
      ) : !me ? (
        <p>Loading upload settings…</p>
      ) : selected && !detail ? (
        <p role={detailError ? "alert" : undefined}>
          {detailError ? errorMessage(detailError) : "Loading artifact…"}
        </p>
      ) : (
        <UploadForm
          key={selected + revision}
          detail={detail}
          limits={me.limits}
          existingIDs={artifacts.map((a) => a.id)}
          allowed={!!selected || me.user.role === "admin" || !!me.creator}
          onBusy={setBusy}
          onDirty={setDirty}
          onReset={() => {
            setDirty(false);
            setSelected("");
            setRevision((n) => n + 1);
          }}
        />
      )}
    </div>
  );
}
function Choice({
  item,
  remove,
  move,
}: {
  item: MediaChoice;
  remove: () => void;
  move?: () => void;
}) {
  const [url, setURL] = useState("");
  useEffect(() => {
    if (!item.file) return;
    const u = URL.createObjectURL(item.file);
    setURL(u);
    return () => URL.revokeObjectURL(u);
  }, [item.file]);
  const src = item.id ? "/media/" + item.id : url;
  return (
    <div className="upload-media-item">
      {item.mediaType.startsWith("video/") ? (
        <video src={src} muted controls preload="metadata" />
      ) : (
        <img src={src} alt={item.file?.name ?? "Selected media"} />
      )}
      <span>{item.file?.name ?? item.id}</span>
      <div>
        {move && (
          <button type="button" onClick={move}>
            Move left
          </button>
        )}
        <button type="button" onClick={remove}>
          Remove
        </button>
      </div>
    </div>
  );
}
function UploadForm({
  detail,
  limits,
  existingIDs,
  allowed,
  onBusy,
  onDirty,
  onReset,
}: {
  detail?: StudioDetail;
  limits: Limits;
  existingIDs: string[];
  allowed: boolean;
  onBusy: (v: boolean) => void;
  onDirty: (v: boolean) => void;
  onReset: () => void;
}) {
  const [values, setValues] = useState(() => initialValues(detail));
  const [html, setHTML] = useState<File>();
  const page = detail?.artifact.pageDraft ?? detail?.artifact.page;
  const { data: library = [] } = studio.useMediaQuery();
  const fromID = (id: string): MediaChoice => ({
    key: id,
    id,
    mediaType: library.find((m) => m.id === id)?.mediaType ?? "image/png",
  });
  const [cover, setCover] = useState<MediaChoice | undefined>(() =>
    page?.cover ? fromID(page.cover) : undefined,
  );
  const [gallery, setGallery] = useState<MediaChoice[]>(() =>
    (page?.gallery ?? []).map(fromID),
  );
  const [attachments, setAttachments] = useState<File[]>([]);
  const [picker, setPicker] = useState<"cover" | "gallery">();
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [stages, setStages] = useState<Stage[]>([]);
  const [result, setResult] = useState<UploadResult>();
  const [busy, setBusy] = useState(false);
  const busyRef = useRef(false),
    touched = useRef(new Set<string>()),
    dirty = useRef(false),
    generation = useRef(0);
  const mediaIDs = useRef(new Map<File, string>());
  const mediaSequence = useRef(0);
  const formRef = useRef<HTMLFormElement>(null);
  const [check] = studio.useCheckReleaseMutation(),
    [create] = studio.useUploadReleaseMutation(),
    [uploadMedia] = studio.useUploadMediaMutation(),
    [attach] = studio.useUploadAttachmentMutation(),
    [patch] = studio.usePatchArtifactMutation(),
    [savePage] = studio.useSaveDraftMutation(),
    [publishPage] = studio.usePublishDraftMutation(),
    [publishRelease] = studio.useReleaseActionMutation();
  const [reload] = studio.useLazyStudioArtifactQuery();
  const locked = busy || !!result?.recovery;
  function change() {
    dirty.current = true;
    onDirty(true);
    setResult(undefined);
  }
  function set<K extends keyof UploadValues>(key: K, value: UploadValues[K]) {
    touched.current.add(key);
    change();
    setValues((v) => ({ ...v, [key]: value }));
  }
  useEffect(() => {
    const prevent = (e: BeforeUnloadEvent) => {
      if (dirty.current || busyRef.current) {
        e.preventDefault();
        e.returnValue = "";
      }
    };
    const navigate = (event: MouseEvent) => {
      const link = (event.target as Element).closest?.("a[href]");
      if (
        !link ||
        event.defaultPrevented ||
        event.ctrlKey ||
        event.metaKey ||
        event.shiftKey ||
        event.button !== 0
      )
        return;
      if (
        busyRef.current ||
        (dirty.current &&
          !window.confirm(
            "Leave this upload and discard unsaved changes? Completed server writes will remain saved.",
          ))
      ) {
        event.preventDefault();
        event.stopPropagation();
      } else {
        dirty.current = false;
      }
    };
    document.addEventListener("click", navigate, true);
    window.addEventListener("beforeunload", prevent);
    return () => {
      generation.current++;
      window.removeEventListener("beforeunload", prevent);
      document.removeEventListener("click", navigate, true);
    };
  }, []);
  async function chooseHTML(file?: File) {
    change();
    setHTML(file);
    const token = ++generation.current;
    if (!file) return;
    const source = await file.slice(0, 65536).text();
    if (generation.current !== token) return;
    const title =
      new DOMParser().parseFromString(source, "text/html").title.trim() ||
      file.name.replace(/\.html?$/i, "");
    setValues((v) => ({
      ...v,
      title: touched.current.has("title") || detail ? v.title : title,
      id: touched.current.has("id") || detail ? v.id : slugify(title),
    }));
  }
  async function send(mode: "check" | "draft" | "publish") {
    if (busyRef.current || result?.recovery || (mode !== "check" && !allowed))
      return;
    const snapshot = {
      ...values,
      id: values.id.trim(),
      title: values.title.trim(),
      version: values.version.trim(),
    };
    const invalid = validateUpload(
      snapshot,
      html,
      cover,
      gallery,
      attachments,
      limits,
      detail,
      existingIDs,
    );
    setErrors(invalid);
    if (Object.keys(invalid).length) {
      formRef.current
        ?.querySelector<HTMLElement>(
          `[data-field="${Object.keys(invalid)[0]}"]`,
        )
        ?.focus();
      return;
    }
    busyRef.current = true;
    setBusy(true);
    onBusy(true);
    setResult(undefined);
    setStages([]);
    setPicker(undefined);
    try {
      const outcome = await submitUpload(
        mode,
        {
          ...snapshot,
          html: html!,
          cover,
          gallery: [...gallery],
          attachments: [...attachments],
          loaded: detail,
        },
        {
          check: (f) => check(f).unwrap(),
          media: (f) => uploadMedia(f).unwrap(),
          create: (f) => create(f).unwrap(),
          attach: (release, form) =>
            attach({ artifact: snapshot.id, release, form }).unwrap(),
          details: (id, fields) => patch({ id, ...fields }).unwrap(),
          page: (id, p) => savePage({ id, ...p }).unwrap(),
          publishPage: (id) => publishPage(id).unwrap(),
          publishRelease: (id) =>
            publishRelease({
              artifact: snapshot.id,
              id,
              action: "publish",
            }).unwrap(),
          reload: (id) => reload(id, false).unwrap(),
        },
        mediaIDs.current,
        setStages,
      );
      setResult(outcome);
      if (outcome.state === "draft" || outcome.state === "published") {
        dirty.current = false;
        onDirty(false);
      }
    } finally {
      busyRef.current = false;
      setBusy(false);
      onBusy(false);
    }
  }
  const field = (
    key: Exclude<keyof UploadValues, "publicDownload">,
    label: string,
    multiline = false,
  ) => (
    <label className="field" key={key}>
      {label}
      {multiline ? (
        <textarea
          data-field={key}
          aria-invalid={!!errors[key]}
          aria-describedby={errors[key] ? `error-${key}` : undefined}
          rows={3}
          value={values[key]}
          onChange={(e) => set(key, e.target.value)}
        />
      ) : (
        <input
          data-field={key}
          aria-invalid={!!errors[key]}
          aria-describedby={errors[key] ? `error-${key}` : undefined}
          value={values[key]}
          readOnly={key === "id" && !!detail}
          onChange={(e) => set(key, e.target.value)}
        />
      )}
      {errors[key] && (
        <small id={`error-${key}`} role="alert">
          {errors[key]}
        </small>
      )}
    </label>
  );
  const fileChoice = (f: File): MediaChoice => ({
    key: `local-${++mediaSequence.current}`,
    file: f,
    mediaType: f.type,
  });
  let manifest: ReturnType<typeof parseManifest> | undefined;
  try {
    manifest = parseManifest(values.manifest);
  } catch {
    /* JSON editor remains available. */
  }
  return (
    <form
      ref={formRef}
      className="upload-layout"
      onSubmit={(e) => {
        e.preventDefault();
        void send("draft");
      }}
    >
      <fieldset disabled={locked} className="upload-fields">
        <section className="upload-panel">
          <h2>01 / Prototype</h2>
          <label
            className="upload-drop"
            onDragOver={(e) => e.preventDefault()}
            onDrop={(e) => {
              e.preventDefault();
              if (!locked) void chooseHTML(e.dataTransfer.files[0]);
            }}
          >
            {html ? html.name : "Drop your HTML file here, or choose a file"}
            <input
              data-field="html"
              type="file"
              accept=".html,.htm,text/html"
              onChange={(e) => void chooseHTML(e.target.files?.[0])}
            />
            <span className="muted small">
              Complete authored HTML · up to {bytes(limits.maxHTMLBytes)}. Build
              ZIP projects first.
            </span>
          </label>
          {errors.html && <p role="alert">{errors.html}</p>}
        </section>
        <section className="upload-panel">
          <h2>02 / Details</h2>
          {detail && (
            <p className="upload-notice">
              Title, description and tags update the live catalog even when
              saving a draft. The release and storefront page have their own
              draft states.
            </p>
          )}
          <div className="upload-two">
            {field("title", "Title")}
            {field("id", "Artifact ID")}
            {field("version", "Version")}
            {field("tags", "Tags, separated by commas")}
          </div>
          {field("description", "Short description", true)}
          {field("changelog", "Release notes", true)}
        </section>
        <section className="upload-panel">
          <h2>03 / What does it need?</h2>
          <div className="upload-capabilities">
            {capabilities.map(([key, label, hint]) => (
              <label key={key}>
                <input
                  type="checkbox"
                  checked={!!manifest && capabilityOn(manifest, key)}
                  disabled={!manifest || !editableCapability(manifest, key)}
                  onChange={(e) =>
                    set(
                      "manifest",
                      JSON.stringify(
                        toggleCapability(manifest!, key, e.target.checked),
                        null,
                        2,
                      ),
                    )
                  }
                />
                <span>
                  <strong>{label}</strong>
                  <small>{hint}</small>
                </span>
              </label>
            ))}
          </div>
          <details>
            <summary>Manifest JSON</summary>
            {field("manifest", "Full capability manifest", true)}
          </details>
          <label className="upload-public">
            <input
              type="checkbox"
              checked={values.publicDownload}
              onChange={(e) => set("publicDownload", e.target.checked)}
            />
            Public download
          </label>
          <p className="small muted">
            Anyone can download this release and its attachments, including
            during early access. Hosted workspace access still follows the
            artifact’s access rules.
          </p>
        </section>
        <section className="upload-panel">
          <h2>04 / Storefront</h2>
          {(["cover", "gallery"] as const).map((kind) => (
            <div className="upload-media" key={kind}>
              <h3>{kind === "cover" ? "Cover" : "Gallery"}</h3>
              <p className="small muted">
                Images or video · {bytes(limits.maxMediaBytes)} each
                {kind === "gallery"
                  ? ` · up to ${limits.maxGalleryItems} items`
                  : ""}
              </p>
              <input
                aria-label={`Upload ${kind}`}
                type="file"
                accept={mediaAccept}
                multiple={kind === "gallery"}
                onChange={(e) => {
                  change();
                  const files = Array.from(e.target.files ?? []).map(
                    fileChoice,
                  );
                  if (kind === "cover") setCover(files[0]);
                  else setGallery((g) => [...g, ...files]);
                  e.target.value = "";
                }}
              />
              <button type="button" onClick={() => setPicker(kind)}>
                Choose from library
              </button>
              <div className="upload-media-list">
                {(kind === "cover" ? (cover ? [cover] : []) : gallery).map(
                  (item, i) => (
                    <Choice
                      key={item.key}
                      item={item.id ? fromID(item.id) : item}
                      remove={() => {
                        change();
                        kind === "cover"
                          ? setCover(undefined)
                          : setGallery((g) => g.filter((_, n) => n !== i));
                      }}
                      move={
                        kind === "gallery" && i > 0
                          ? () => {
                              change();
                              setGallery((g) => {
                                const next = [...g];
                                [next[i - 1], next[i]] = [next[i], next[i - 1]];
                                return next;
                              });
                            }
                          : undefined
                      }
                    />
                  ),
                )}
              </div>
              {errors[kind] && <p role="alert">{errors[kind]}</p>}
            </div>
          ))}
          {picker && (
            <MediaPicker
              onClose={() => setPicker(undefined)}
              onPick={(m) => {
                change();
                const item = { key: m.id, id: m.id, mediaType: m.mediaType };
                picker === "cover"
                  ? setCover(item)
                  : setGallery((g) =>
                      g.some((x) => x.id === m.id) ? g : [...g, item],
                    );
                setPicker(undefined);
              }}
            />
          )}
          <MarkdownField
            label="The write-up (Markdown)"
            value={values.body}
            onChange={(v) => set("body", v)}
            rows={8}
          />
          {errors.body && <p role="alert">{errors.body}</p>}
        </section>
        <section className="upload-panel">
          <h2>05 / Attachments</h2>
          <p className="muted small">
            Source files, notes or extras · up to {limits.maxAttachments} files
            · {bytes(limits.maxAttachmentBytes)} each.
          </p>
          <input
            aria-label="Attachments"
            type="file"
            multiple
            onChange={(e) => {
              change();
              const files = Array.from(e.target.files ?? []);
              setAttachments((a) => [...a, ...files]);
              e.target.value = "";
            }}
          />
          {attachments.map((f, i) => (
            <p key={i}>
              {f.name} <span className="muted">{bytes(f.size)}</span>{" "}
              <button
                type="button"
                onClick={() => {
                  change();
                  setAttachments((a) => a.filter((_, n) => n !== i));
                }}
              >
                Remove
              </button>
            </p>
          ))}
          {errors.attachments && <p role="alert">{errors.attachments}</p>}
        </section>
      </fieldset>
      <aside className="upload-summary">
        <h2>Ready when you are.</h2>
        <p>{values.title || "Your next artifact"}</p>
        <p className="muted small">
          {values.version || "Choose a version"} ·{" "}
          {html ? bytes(html.size) : "No HTML selected"}
        </p>
        <p className="small">
          Save a draft to review it in Studio, or publish the page and release
          together.
        </p>
        {!allowed && (
          <p role="alert">
            Ask an administrator to link your creator profile before uploading a
            new artifact.
          </p>
        )}
        <div className="upload-actions">
          <button
            type="button"
            disabled={locked}
            onClick={() => void send("check")}
          >
            Check HTML
          </button>
          <button type="submit" disabled={locked || !allowed}>
            Save draft
          </button>
          <button
            type="button"
            className="btn"
            disabled={locked || !allowed}
            onClick={() => void send("publish")}
          >
            Publish artifact ↗
          </button>
        </div>
        {Object.keys(errors).length > 0 && (
          <p role="alert">Please fix the highlighted fields.</p>
        )}
        <div aria-live="polite">
          {stages.length > 0 && (
            <ol className="upload-progress">
              {stages.map((s, i) => (
                <li key={i} data-state={s.state}>
                  {s.state === "done" ? "✓" : s.state === "failed" ? "!" : "…"}{" "}
                  {s.label}
                </li>
              ))}
            </ol>
          )}
          {result && (
            <div className="upload-result">
              <strong>
                {
                  {
                    checked: "HTML checked",
                    blocked: "HTML needs changes",
                    draft: "Draft saved",
                    published: "Published",
                    failed: "Upload stopped",
                  }[result.state]
                }
              </strong>
              {result.error != null && <p>{errorMessage(result.error)}</p>}
              {result.checks.map((c, i) => (
                <p key={i} className="small">
                  {c.message}
                </p>
              ))}
              {result.recovery && (
                <>
                  <p>
                    {result.release
                      ? `Release ${result.release.version}: ${result.release.state}.`
                      : "The creation request may have been saved. Check Studio before uploading again."}
                  </p>
                  {result.pagePublished && (
                    <p>The storefront page is published.</p>
                  )}
                  {result.state === "failed" && (
                    <p>
                      Completed steps remain saved. Finish or correct this
                      release in Studio; this form will not replay the upload.
                    </p>
                  )}
                  <Link to={`/studio/a/${values.id}?tab=releases`}>
                    Open in Studio →
                  </Link>
                  {result.state === "published" && (
                    <p>
                      <Link to={`/apps/${values.id}`}>View artifact →</Link>
                    </p>
                  )}
                  <button type="button" onClick={onReset}>
                    Upload another
                  </button>
                </>
              )}
            </div>
          )}
        </div>
      </aside>
    </form>
  );
}
