import test from "node:test";
import assert from "node:assert/strict";
import { File as NodeFile } from "node:buffer";
import {
  initialValues,
  toggleCapability,
  nextVersion,
  validateUpload,
  type UploadInput,
} from "../ui/catalog/uploader-model";
import { submitUpload, type UploadAPI } from "../ui/catalog/uploader-submit";
const file = (name: string) =>
  new NodeFile(["<!doctype html><title>Demo</title>"], name, {
    type: "text/html",
  }) as unknown as File;
const input = (): UploadInput => ({
  ...initialValues(),
  id: "demo",
  title: "Demo",
  html: file("demo.html"),
  gallery: [],
  attachments: [],
  body: "# Demo",
});
function fixture(fail = "") {
  const calls: string[] = [];
  let published = false;
  const release = { id: "rel_1", version: "0.1.0", state: "draft" };
  async function step(name: string) {
    calls.push(name);
    if (name === fail) throw new Error(name);
  }
  const api = {
    check: async () => {
      await step("check");
      return { checks: [], blocking: false };
    },
    media: async () => {
      await step("media");
      return { id: "media_1" };
    },
    create: async () => {
      await step("create");
      return release;
    },
    attach: async () => {
      await step("attach");
    },
    details: async () => {
      await step("details");
    },
    page: async () => {
      await step("page");
    },
    publishPage: async () => {
      await step("publishPage");
    },
    publishRelease: async () => {
      published = true;
      await step("publishRelease");
    },
    reload: async () => {
      await step("reload");
      return {
        releases: [{ ...release, state: published ? "published" : "draft" }],
      };
    },
  } as unknown as UploadAPI;
  return { api, calls };
}
test("capability edits preserve opaque declarations and existing scoped options", () => {
  const original = {
    sample: { model: "default", images: true },
    db: { tables: ["notes"] },
    slop_boutique: { artifacts: true },
  };
  const updated = toggleCapability(original, "images", false);
  assert.deepEqual(updated, {
    sample: { model: "default" },
    db: { tables: ["notes"] },
    slop_boutique: { artifacts: true },
  });
  assert.equal(original.sample.images, true);
  assert.deepEqual(toggleCapability(original, "db", true).db, original.db);
});
test("version suggestions and byte limits match authored input", () => {
  assert.equal(
    nextVersion([{ version: "1.9.0" }, { version: "1.10.2" }]),
    "1.10.3",
  );
  assert.equal(nextVersion([{ version: "preview" }]), "");
  const i = input();
  i.title = "界".repeat(41);
  const errors = validateUpload(i, i.html, undefined, [], [], {
    maxHTMLBytes: 1000,
    maxMediaBytes: 1000,
    maxAttachmentBytes: 1000,
    maxGalleryItems: 24,
    maxAttachments: 20,
  });
  assert.match(errors.title, /120/);
});
test("checking is read-only; publish completes page before release", async () => {
  const f = fixture();
  assert.equal(
    (await submitUpload("check", input(), f.api, new Map(), () => {})).state,
    "checked",
  );
  assert.deepEqual(f.calls, ["check"]);
  f.calls.length = 0;
  assert.equal(
    (await submitUpload("publish", input(), f.api, new Map(), () => {})).state,
    "published",
  );
  assert.deepEqual(f.calls, [
    "check",
    "create",
    "page",
    "publishPage",
    "publishRelease",
  ]);
});
test("attachment failure retains draft and prevents later mutations", async () => {
  const f = fixture("attach"),
    i = input();
  i.attachments = [file("notes.txt")];
  const out = await submitUpload("publish", i, f.api, new Map(), () => {});
  assert.equal(out.state, "failed");
  assert.equal(out.recovery, true);
  assert.equal(out.release?.id, "rel_1");
  assert.deepEqual(f.calls, ["check", "create", "attach", "reload"]);
});
test("lost creation response reconciles without retry; lost final response confirms publication", async () => {
  for (const failed of ["create", "publishRelease"]) {
    const f = fixture(failed),
      out = await submitUpload("publish", input(), f.api, new Map(), () => {});
    assert.equal(out.state, failed === "create" ? "failed" : "published");
    assert.equal(out.release?.id, "rel_1");
    assert.equal(f.calls.filter((x) => x === "create").length, 1);
    assert.equal(f.calls.at(-1), "reload");
  }
});
test("successful media uploads are retained across a pre-creation retry", async () => {
  const f = fixture(),
    i = input(),
    cache = new Map<File, string>();
  i.cover = { key: "1", file: file("cover.png"), mediaType: "image/png" };
  i.gallery = [{ key: "2", file: file("gallery.png"), mediaType: "image/png" }];
  let count = 0;
  f.api.media = async () => {
    if (++count === 2) throw new Error("temporary failure");
    return { id: "m" + count } as any;
  };
  const first = await submitUpload("draft", i, f.api, cache, () => {});
  assert.equal(first.recovery, false);
  assert.equal(cache.size, 1);
  assert.equal(
    (await submitUpload("draft", i, f.api, cache, () => {})).state,
    "draft",
  );
  assert.equal(count, 3);
});
