import type { Check, Release } from "./api";
import type { Media, Page, StudioDetail, UploadedRelease } from "./studio-api";
import {
  normalizedPage,
  parseTags,
  releaseForm,
  type MediaChoice,
  type UploadInput,
} from "./uploader-model";

export type Stage = { label: string; state: "running" | "done" | "failed" };
export type UploadResult = {
  state: "checked" | "blocked" | "draft" | "published" | "failed";
  checks: Check[];
  release?: Pick<Release, "id" | "state" | "version">;
  recovery: boolean;
  error?: unknown;
  pagePublished: boolean;
};
export type UploadAPI = {
  check(form: FormData): Promise<{ checks: Check[]; blocking: boolean }>;
  media(form: FormData): Promise<Media>;
  create(form: FormData): Promise<UploadedRelease>;
  attach(release: string, form: FormData): Promise<unknown>;
  details(
    id: string,
    patch: { title: string; description: string; tags: string[] },
  ): Promise<unknown>;
  page(id: string, page: Page): Promise<unknown>;
  publishPage(id: string): Promise<unknown>;
  publishRelease(id: string): Promise<unknown>;
  reload(id: string): Promise<StudioDetail>;
};

// Every successful mutation is durable. Once creation was attempted, recover
// through Studio rather than replaying a possibly committed write.
export async function submitUpload(
  mode: "check" | "draft" | "publish",
  input: UploadInput,
  api: UploadAPI,
  mediaIDs: Map<File, string>,
  progress: (stages: Stage[]) => void,
): Promise<UploadResult> {
  const result: UploadResult = {
    state: "failed",
    checks: [],
    recovery: false,
    pagePublished: false,
  };
  const stages: Stage[] = [];
  async function stage<T>(label: string, fn: () => Promise<T>): Promise<T> {
    const row: Stage = { label, state: "running" };
    stages.push(row);
    const notify = () => progress(stages.map((s) => ({ ...s })));
    notify();
    try {
      const out = await fn();
      row.state = "done";
      notify();
      return out;
    } catch (error) {
      row.state = "failed";
      notify();
      throw error;
    }
  }
  async function media(item?: MediaChoice): Promise<string> {
    if (!item) return "";
    if (item.id) return item.id;
    const file = item.file!;
    if (mediaIDs.has(file)) return mediaIDs.get(file)!;
    const form = new FormData();
    form.set("file", file); // alt can be edited in the library
    const saved = await stage("Uploading " + file.name, () => api.media(form));
    mediaIDs.set(file, saved.id);
    return saved.id;
  }
  try {
    const checked = await stage("Checking HTML", () =>
      api.check(releaseForm(input)),
    );
    result.checks = checked.checks;
    if (checked.blocking) return { ...result, state: "blocked" };
    if (mode === "check") return { ...result, state: "checked" };
    const page: Page = {
      body: input.body,
      cover: await media(input.cover),
      gallery: [],
    };
    for (const item of input.gallery) page.gallery.push(await media(item));
    result.recovery = true;
    result.release = await stage("Saving release " + input.version, () =>
      api.create(releaseForm(input)),
    );
    for (const file of input.attachments) {
      const form = new FormData();
      form.set("file", file);
      await stage("Attaching " + file.name, () =>
        api.attach(result.release!.id, form),
      );
    }
    const details = {
      title: input.title,
      description: input.description,
      tags: parseTags(input.tags),
    };
    const loaded = input.loaded?.artifact;
    if (
      loaded &&
      (loaded.title !== details.title ||
        loaded.description !== details.description ||
        JSON.stringify(loaded.tags) !== JSON.stringify(details.tags))
    )
      await stage("Saving catalog details", () =>
        api.details(input.id, details),
      );
    const changed =
      JSON.stringify(normalizedPage(page)) !==
      JSON.stringify(normalizedPage(loaded?.pageDraft ?? loaded?.page));
    if (changed)
      await stage("Saving page draft", () => api.page(input.id, page));
    if (mode === "publish") {
      if (changed || loaded?.pageDraft) {
        await stage("Publishing storefront page", () =>
          api.publishPage(input.id),
        );
        result.pagePublished = true;
      }
      await stage("Publishing release", () =>
        api.publishRelease(result.release!.id),
      );
      result.release = { ...result.release, state: "published" };
    }
    return { ...result, state: mode === "publish" ? "published" : "draft" };
  } catch (error) {
    result.error = error;
    const errorChecks = (error as { data?: { checks?: Check[] } })?.data
      ?.checks;
    if (errorChecks) result.checks = errorChecks;
    if (result.recovery) {
      try {
        const actual = await api.reload(input.id);
        const release = actual.releases.find((r) =>
          result.release
            ? r.id === result.release.id
            : r.version === input.version,
        );
        if (release) result.release = release;
        // A lost final response does not undo publication. All prior steps
        // completed if we reached the final publish stage.
        if (
          release?.state === "published" &&
          stages.at(-1)?.label === "Publishing release"
        ) {
          stages.at(-1)!.state = "done";
          progress(stages.map((s) => ({ ...s })));
          return { ...result, state: "published", error: undefined };
        }
      } catch {
        /* Keep the uncertainty explicit; never replay the creation. */
      }
    }
    return result;
  }
}
