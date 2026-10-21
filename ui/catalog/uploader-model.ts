import type { Release } from "./api";
import type { Limits, Page, StudioDetail } from "./studio-api";

export type Manifest = Record<string, unknown>;
export type MediaChoice = {
  key: string;
  id?: string;
  file?: File;
  mediaType: string;
};
export type UploadValues = {
  id: string;
  title: string;
  version: string;
  tags: string;
  description: string;
  changelog: string;
  manifest: string;
  publicDownload: boolean;
  body: string;
};
export type UploadInput = UploadValues & {
  html: File;
  cover?: MediaChoice;
  gallery: MediaChoice[];
  attachments: File[];
  loaded?: StudioDetail;
};
export const capabilities = [
  ["sample", "Model access", "Call the AI model"],
  ["images", "Images to the model", "Send up to four images with a prompt"],
  ["db", "Workspace database", "JSON documents saved with the workspace"],
  ["room", "Rooms & presence", "Live multi-user events"],
  ["user", "User profile", "The viewer’s name"],
  ["assets", "File uploads", "Images and files stored with the workspace"],
  ["comments", "Comments", "Threaded comments"],
  ["downloads", "App-generated downloads", "Let the app save files"],
] as const;
export const mediaAccept =
  "image/png,image/jpeg,image/gif,image/webp,video/mp4,video/webm";
export const utf8Size = (text: string) => new TextEncoder().encode(text).length;
export const parseTags = (text: string) => [
  ...new Set(
    text
      .split(",")
      .map((t) => t.trim().toLowerCase())
      .filter(Boolean),
  ),
];
export const slugify = (text: string) =>
  text
    .toLowerCase()
    .normalize("NFKD")
    .replace(/[\u0300-\u036f]/g, "")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+/, "")
    .slice(0, 64)
    .replace(/-+$/, "");
export function nextVersion(releases: Pick<Release, "version">[]) {
  const versions = releases.map((r) => r.version);
  if (!versions.length) return "0.1.0";
  const numeric = versions
    .filter((v) => /^\d+\.\d+\.\d+$/.test(v))
    .sort((a, b) => {
      const x = a.split(".").map(Number),
        y = b.split(".").map(Number);
      return x[0] - y[0] || x[1] - y[1] || x[2] - y[2];
    });
  if (!numeric.length) return "";
  const [major, minor, patch] = numeric.at(-1)!.split(".").map(Number);
  return `${major}.${minor}.${patch + 1}`;
}
export function parseManifest(raw: string): Manifest {
  const value: unknown = JSON.parse(raw);
  if (!value || typeof value !== "object" || Array.isArray(value))
    throw new Error("Manifest must be a JSON object.");
  return value as Manifest;
}
const object = (v: unknown): v is Manifest =>
  !!v && typeof v === "object" && !Array.isArray(v);
export function editableCapability(manifest: Manifest, key: string) {
  const value = manifest[key === "images" ? "sample" : key];
  return (
    value === undefined ||
    (key === "downloads" ? typeof value === "boolean" : object(value))
  );
}
export function capabilityOn(manifest: Manifest, key: string) {
  return key === "images"
    ? object(manifest.sample) && manifest.sample.images === true
    : key === "downloads"
      ? manifest.downloads === true
      : key in manifest;
}
export function toggleCapability(
  original: Manifest,
  key: string,
  on: boolean,
): Manifest {
  if (!editableCapability(original, key))
    throw new Error("Edit this declaration in Manifest JSON.");
  const next = structuredClone(original);
  if (key === "images") {
    if (on)
      next.sample = { ...((next.sample as Manifest) ?? {}), images: true };
    else if (object(next.sample)) delete next.sample.images;
  } else if (on) {
    if (key === "downloads") next.downloads = true;
    else next[key] ??= key === "user" ? { scopes: ["profile"] } : {};
  } else delete next[key];
  return next;
}
export const normalizedPage = (page?: Page): Page => ({
  body: page?.body ?? "",
  cover: page?.cover ?? "",
  gallery: page?.gallery ?? [],
});
export function initialValues(detail?: StudioDetail): UploadValues {
  const a = detail?.artifact;
  const published = detail?.releases
    .filter((r) => r.state === "published")
    .sort((a, b) => (b.publishedAt ?? "").localeCompare(a.publishedAt ?? ""));
  const latest =
    published?.[0] ?? detail?.releases.find((r) => r.state === "draft");
  return {
    id: a?.id ?? "",
    title: a?.title ?? "",
    description: a?.description ?? "",
    tags: a?.tags.join(", ") ?? "",
    version: nextVersion(detail?.releases ?? []),
    changelog: "",
    body: (a?.pageDraft ?? a?.page)?.body ?? "",
    manifest: JSON.stringify(latest?.manifest ?? { sample: {} }, null, 2),
    publicDownload: latest?.freeSample ?? false,
  };
}
export function validateUpload(
  values: UploadValues,
  html: File | undefined,
  cover: MediaChoice | undefined,
  gallery: MediaChoice[],
  attachments: File[],
  limits: Limits,
  detail?: StudioDetail,
  existingIDs: string[] = [],
) {
  const errors: Record<string, string> = {};
  if (!/^[a-z0-9][a-z0-9_-]{0,63}$/.test(values.id.trim()))
    errors.id =
      "Use 1–64 lowercase letters, digits, - or _; start with a letter or digit.";
  else if (!detail && existingIDs.includes(values.id.trim()))
    errors.id =
      "This artifact exists. Select it above to upload a new version.";
  if (!values.title.trim() || utf8Size(values.title.trim()) > 120)
    errors.title = "Enter a title of at most 120 UTF-8 bytes.";
  if (!/^[A-Za-z0-9][A-Za-z0-9._+-]{0,39}$/.test(values.version.trim()))
    errors.version = "Enter a version of 1–40 letters, digits, . _ + or -.";
  else if (detail?.releases.some((r) => r.version === values.version.trim()))
    errors.version = "This version already exists. Choose a new version.";
  if (utf8Size(values.description) > 2000)
    errors.description = "Description exceeds 2,000 UTF-8 bytes.";
  if (utf8Size(values.changelog) > 8000)
    errors.changelog = "Release notes exceed 8,000 UTF-8 bytes.";
  if (parseTags(values.tags).length > 12) errors.tags = "Use at most 12 tags.";
  if (utf8Size(values.body) > 100 * 1024)
    errors.body = "Write-up exceeds 100 KiB.";
  try {
    parseManifest(values.manifest);
  } catch {
    errors.manifest = "Enter a valid JSON object.";
  }
  if (utf8Size(values.manifest) > 64 * 1024)
    errors.manifest = "Manifest exceeds 64 KiB.";
  if (!html || !html.size)
    errors.html = "Choose a complete authored HTML file.";
  else if (html.size > limits.maxHTMLBytes)
    errors.html = "HTML exceeds the upload size limit.";
  else if (!/\.html?$/i.test(html.name))
    errors.html = "Upload an HTML file; build ZIP projects locally first.";
  if (gallery.length > limits.maxGalleryItems)
    errors.gallery = `Use at most ${limits.maxGalleryItems} gallery items.`;
  for (const [field, items] of [
    ["cover", cover ? [cover] : []],
    ["gallery", gallery],
  ] as const) {
    for (const item of items)
      if (item.file) {
        if (item.file.size > limits.maxMediaBytes)
          errors[field] = `${item.file.name} exceeds the media size limit.`;
        else if (!mediaAccept.split(",").includes(item.file.type))
          errors[field] = "Choose PNG, JPEG, GIF, WebP, MP4 or WebM media.";
      }
  }
  if (attachments.length > limits.maxAttachments)
    errors.attachments = `Use at most ${limits.maxAttachments} attachments.`;
  for (const file of attachments)
    if (file.size > limits.maxAttachmentBytes)
      errors.attachments = `${file.name} exceeds the attachment size limit.`;
  return errors;
}
export function releaseForm(input: UploadInput) {
  const form = new FormData();
  for (const key of ["title", "description", "version", "changelog"] as const)
    form.set(key, input[key]);
  form.set("artifactId", input.id);
  form.set("tags", parseTags(input.tags).join(","));
  form.set("manifestJson", input.manifest);
  form.set("freeSample", String(input.publicDownload));
  form.set("html", input.html, input.html.name);
  return form;
}
