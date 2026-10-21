import React, { useEffect, useId, useRef, useState } from "react";
import { Media, studio } from "./studio-api";
import { bytes, useAct } from "./ui";

export function Still({ id, className }: { id: string; className?: string }) {
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

function useMarkdownPreview(source: string) {
  const [render] = studio.useMarkdownMutation();
  const [html, setHTML] = useState("");
  useEffect(() => {
    let current = true;
    const t = setTimeout(() => {
      render(source)
        .unwrap()
        .then((r) => {
          if (current) setHTML(r.html);
        })
        .catch(() => undefined);
    }, 300);
    return () => {
      current = false;
      clearTimeout(t);
    };
  }, [source, render]);
  return html;
}

export function MarkdownField({
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
  const id = useId();
  return (
    <div className="md-editor">
      <div className="field">
        <label htmlFor={id}>{label}</label>
        <textarea
          id={id}
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

export function MediaThumb({
  id,
  onRemove,
}: {
  id: string;
  onRemove?: () => void;
}) {
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

export function MediaPicker({
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

export const isVideo = (m: Media) => m.mediaType.startsWith("video/");

export function MediaPreview({ m }: { m: Media }) {
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

export function Uploader() {
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
