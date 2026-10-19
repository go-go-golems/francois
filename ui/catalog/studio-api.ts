import { api, Check, Instance, Release } from "./api";

// Studio endpoints (/api/studio): content management for creators and
// administrators. Injected into the shared API so they reuse its CSRF header.

export type Page = { body: string; cover?: string; gallery: string[] };
export type StudioArtifact = {
  id: string;
  title: string;
  description: string;
  tags: string[];
  visibility: "listed" | "hidden";
  page: Page;
  pageDraft?: Page;
  featured: boolean;
  sortOrder: number;
  earlyAccessUntil?: string;
  demoEnabled: boolean;
  createdAt: string;
  updatedAt: string;
};
export type ArtifactRow = {
  id: string;
  title: string;
  description: string;
  tags: string[];
  visibility: string;
  featured: boolean;
  sortOrder: number;
  earlyAccessUntil?: string;
  demoEnabled: boolean;
  cover: string;
  hasDraft: boolean;
  creators: string[];
  releases: number;
  draftReleases: number;
  latestVersion: string;
  updatedAt: string;
};
export type Media = { id: string; mediaType: string; size: number; width: number; height: number; alt: string; uploadedBy: string; createdAt: string; poster: string };
export type Limits = { maxMediaBytes: number; maxAttachmentBytes: number };
export type Link = { label: string; url: string };
export type Creator = { id: string; slug: string; name: string; bio: string; avatar?: string; links: Link[]; userId?: string; sortOrder: number };
export type Post = {
  id: string;
  slug: string;
  title: string;
  summary: string;
  body: string;
  cover?: string;
  authorId?: string;
  tags: string[];
  state: "draft" | "published";
  publishedAt?: string;
  createdBy: string;
  updatedAt: string;
};
export type ArtifactPatch = Partial<Pick<StudioArtifact, "title" | "description" | "tags" | "visibility" | "featured" | "sortOrder" | "earlyAccessUntil" | "demoEnabled">> & {
  creators?: string[];
};

const base = api.enhanceEndpoints({ addTagTypes: ["StudioArtifacts", "StudioArtifact", "Media", "Posts", "Post", "Creators", "StudioMe"] });

export const studio = base.injectEndpoints({
  endpoints: (b) => ({
    studioMe: b.query<{ user: { id: string; name: string; role: string }; creator: Creator | null; limits: Limits }, void>({
      query: () => "api/studio/me",
      providesTags: ["StudioMe"],
    }),
    studioArtifacts: b.query<ArtifactRow[], void>({ query: () => "api/studio/artifacts", providesTags: ["StudioArtifacts"] }),
    studioArtifact: b.query<{ artifact: StudioArtifact; creators: string[]; releases: Release[] }, string>({
      query: (id) => `api/studio/artifacts/${id}`,
      providesTags: (_r, _e, id) => [{ type: "StudioArtifact", id }],
    }),
    createArtifact: b.mutation<StudioArtifact, { id: string; title: string; description?: string }>({
      query: (body) => ({ url: "api/studio/artifacts", method: "POST", body }),
      invalidatesTags: ["StudioArtifacts"],
    }),
    patchArtifact: b.mutation<StudioArtifact, { id: string } & ArtifactPatch>({
      query: ({ id, ...body }) => ({ url: `api/studio/artifacts/${id}`, method: "PATCH", body }),
      invalidatesTags: (_r, _e, a) => ["StudioArtifacts", { type: "StudioArtifact", id: a.id }],
    }),
    saveDraft: b.mutation<unknown, { id: string } & Page>({
      query: ({ id, ...body }) => ({ url: `api/studio/artifacts/${id}/draft`, method: "PUT", body }),
      invalidatesTags: (_r, _e, a) => ["StudioArtifacts", { type: "StudioArtifact", id: a.id }],
    }),
    publishDraft: b.mutation<unknown, string>({
      query: (id) => ({ url: `api/studio/artifacts/${id}/draft/publish`, method: "POST" }),
      invalidatesTags: (_r, _e, id) => ["StudioArtifacts", { type: "StudioArtifact", id }],
    }),
    discardDraft: b.mutation<unknown, string>({
      query: (id) => ({ url: `api/studio/artifacts/${id}/draft`, method: "DELETE" }),
      invalidatesTags: (_r, _e, id) => ["StudioArtifacts", { type: "StudioArtifact", id }],
    }),
    checkRelease: b.mutation<{ checks: Check[]; blocking: boolean }, FormData>({
      query: (body) => ({ url: "api/studio/releases/check", method: "POST", body }),
    }),
    uploadRelease: b.mutation<Release, FormData>({
      query: (form) => ({ url: "api/studio/releases", method: "POST", body: form }),
      invalidatesTags: ["StudioArtifacts", "StudioArtifact"],
    }),
    releaseAction: b.mutation<unknown, { artifact: string; id: string; action: "publish" | "withdraw" | "delete"; suspendPinned?: boolean }>({
      query: ({ id, action, suspendPinned }) =>
        action === "delete"
          ? { url: `api/studio/releases/${id}`, method: "DELETE" }
          : { url: `api/studio/releases/${id}/${action}`, method: "POST", body: action === "withdraw" ? { suspendPinned } : undefined },
      invalidatesTags: (_r, _e, a) => ["StudioArtifacts", { type: "StudioArtifact", id: a.artifact }, "Catalog"],
    }),
    uploadAttachment: b.mutation<unknown, { artifact: string; release: string; form: FormData }>({
      query: ({ release, form }) => ({ url: `api/studio/releases/${release}/attachments`, method: "POST", body: form }),
      invalidatesTags: (_r, _e, a) => [{ type: "StudioArtifact", id: a.artifact }],
    }),
    patchAttachment: b.mutation<unknown, { artifact: string; id: string; label?: string; position?: number }>({
      query: ({ id, label, position }) => ({ url: `api/studio/attachments/${id}`, method: "PATCH", body: { label, position } }),
      invalidatesTags: (_r, _e, a) => [{ type: "StudioArtifact", id: a.artifact }],
    }),
    deleteAttachment: b.mutation<unknown, { artifact: string; id: string }>({
      query: ({ id }) => ({ url: `api/studio/attachments/${id}`, method: "DELETE" }),
      invalidatesTags: (_r, _e, a) => [{ type: "StudioArtifact", id: a.artifact }],
    }),
    previewRelease: b.mutation<Instance, string>({
      query: (id) => ({ url: `api/studio/releases/${id}/preview`, method: "POST" }),
    }),
    media: b.query<Media[], void>({ query: () => "api/studio/media", providesTags: ["Media"] }),
    uploadMedia: b.mutation<Media, FormData>({
      query: (body) => ({ url: "api/studio/media", method: "POST", body }),
      invalidatesTags: ["Media"],
    }),
    patchMedia: b.mutation<unknown, { id: string; alt?: string; poster?: string }>({
      query: ({ id, ...body }) => ({ url: `api/studio/media/${id}`, method: "PATCH", body }),
      invalidatesTags: ["Media"],
    }),
    deleteMedia: b.mutation<unknown, string>({
      query: (id) => ({ url: `api/studio/media/${id}`, method: "DELETE" }),
      invalidatesTags: ["Media"],
    }),
    posts: b.query<Post[], void>({ query: () => "api/studio/posts", providesTags: ["Posts"] }),
    post: b.query<Post, string>({ query: (id) => `api/studio/posts/${id}`, providesTags: (_r, _e, id) => [{ type: "Post", id }] }),
    createPost: b.mutation<Post, Partial<Post>>({
      query: (body) => ({ url: "api/studio/posts", method: "POST", body }),
      invalidatesTags: ["Posts"],
    }),
    patchPost: b.mutation<Post, Partial<Post> & { id: string }>({
      query: ({ id, ...body }) => ({ url: `api/studio/posts/${id}`, method: "PATCH", body }),
      invalidatesTags: (_r, _e, p) => ["Posts", { type: "Post", id: p.id }],
    }),
    postAction: b.mutation<unknown, { id: string; action: "publish" | "unpublish" | "delete" }>({
      query: ({ id, action }) => (action === "delete" ? { url: `api/studio/posts/${id}`, method: "DELETE" } : { url: `api/studio/posts/${id}/${action}`, method: "POST" }),
      invalidatesTags: (_r, _e, p) => ["Posts", { type: "Post", id: p.id }],
    }),
    creators: b.query<Creator[], void>({ query: () => "api/studio/creators", providesTags: ["Creators"] }),
    createCreator: b.mutation<Creator, Partial<Creator>>({
      query: (body) => ({ url: "api/studio/creators", method: "POST", body }),
      invalidatesTags: ["Creators", "StudioMe"],
    }),
    patchCreator: b.mutation<Creator, Partial<Creator> & { id: string }>({
      query: ({ id, ...body }) => ({ url: `api/studio/creators/${id}`, method: "PATCH", body }),
      invalidatesTags: ["Creators", "StudioMe"],
    }),
    deleteCreator: b.mutation<unknown, string>({
      query: (id) => ({ url: `api/studio/creators/${id}`, method: "DELETE" }),
      invalidatesTags: ["Creators", "StudioMe"],
    }),
    studioPlans: b.query<{ key: string; name: string }[], void>({ query: () => "api/studio/plans" }),
    markdown: b.mutation<{ html: string }, string>({
      query: (markdown) => ({ url: "api/studio/markdown", method: "POST", body: { markdown } }),
    }),
  }),
});
