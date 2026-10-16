import { createApi, fetchBaseQuery } from "@reduxjs/toolkit/query/react";

export type Decision = {
  allowed: boolean;
  reason: string;
  until?: string;
  status?: string;
  cancelScheduled: boolean;
};
// Personal API tokens. A scope selects an API surface; it never exceeds the
// owner's role.
export type TokenScope = "studio" | "account" | "admin:read" | "admin:write";
export type Token = { id: string; name: string; scopes: TokenScope[]; createdAt: string; lastUsedAt?: string; expiresAt?: string; revokedAt?: string };

export type SessionUser = {
  id: string;
  name: string;
  email: string;
  emailVerified: boolean;
  role: "subscriber" | "creator" | "admin" | "guest";
  disabled: boolean;
};
export type Session = {
  authenticated: boolean;
  loginAvailable: boolean;
  signupAvailable: boolean;
  billingEnabled: boolean;
  appDomain: string;
  brand?: string;
  typeCredit?: boolean;
  user?: SessionUser;
  csrfToken?: string;
  access?: Decision;
  plan?: { key: string; name: string; rank: number; source: string; paid: boolean; instances: number };
};
export type Plan = {
  key: string;
  name: string;
  rank: number;
  description: string;
  highlights: string[];
  public: boolean;
  prices: { interval: "month" | "year"; priceId: string; label: string }[];
  instances: number;
  membersPerInstance: number;
  monthlyCredits: number;
  dailyCredits: number;
  models: string[];
};
export type Entitlement = { plan: Plan; source: string; decision: Decision };
export type ReleaseFile = {
  id: string;
  role: "html" | "manifest";
  name: string;
  mediaType: string;
  size: number;
  sha256: string;
};
export type Attachment = { id: string; name: string; label: string; mediaType: string; size: number; sha256: string; position: number; href: string; createdAt?: string };
export type Release = {
  id: string;
  artifactId: string;
  version: string;
  changelog: string;
  state: "draft" | "published" | "withdrawn";
  publishedAt?: string;
  createdAt?: string;
  withdrawnAt?: string;
  capabilities: string[];
  files: ReleaseFile[];
  attachments: Attachment[];
  downloadAccess: "subscriber_required" | "free_sample";
  freeSample?: boolean;
  pinnedInstances?: number;
  manifest?: Record<string, unknown>;
  checks?: Check[];
};
export type CatalogEntry = {
  id: string;
  title: string;
  description: string;
  tags: string[];
  updatedAt: string;
  releases: number;
  latest: Release;
};
export type ArtifactDetail = {
  id: string;
  title: string;
  description: string;
  tags: string[];
  releases: Release[];
};
export type Member = {
  userId: string;
  name: string;
  email?: string;
  role: "owner" | "editor" | "viewer";
  createdAt: string;
};
export type Instance = {
  id: string;
  title: string;
  kind: "workspace" | "preview" | "demo";
  state: "active" | "suspended";
  role?: "owner" | "editor" | "viewer";
  artifactId: string;
  version: string;
  releaseId: string;
  ownerName: string;
  ownerUserId: string;
  members: number;
  origin: string;
  createdAt: string;
  updatedAt: string;
  expiresAt?: string;
  membersList?: Member[];
  release?: Release;
  upgrades?: Release[];
  launchBlocked?: string;
  activeViews?: number;
  usage?: {
    documents: number;
    documentBytes: number;
    assets: number;
    assetBytes: number;
    activeGrants: number;
  };
  recentFailures?: ModelCall[];
  audit?: AuditEntry[];
};
export type Subscription = {
  id: string;
  priceId: string;
  status: string;
  accessUntil?: string;
  trialUntil?: string;
  cancelAtPeriodEnd: boolean;
  canceledAt?: string;
  refreshedAt: string;
};
export type Allowance = {
  period: string;
  included: number;
  adjustments: number;
  reserved: number;
  used: number;
  remaining: number;
  resetsAt: string;
  usedToday: number;
  daily: number;
};
export type AccessGrant = {
  id: string;
  until: string;
  reason: string;
  grantedBy: string;
  createdAt: string;
  revokedAt?: string;
};
export type Account = {
  user: SessionUser & { createdAt: string };
  access: Decision;
  entitlement: Entitlement;
  subscriptions: Subscription[];
  complimentary: AccessGrant[];
  allowance: Allowance;
  models: { selector: string; label: string; credits: number; thinking: string[] }[];
  hasCustomer: boolean;
  checkouts: { id: string; state: string; createdAt: string }[];
  sessions: number;
  billingEnabled: boolean;
  identityIssuer: string;
};
export type Check = { code: string; level: "error" | "warning" | "info"; message: string };
export type AdminUser = {
  id: string;
  issuer: string;
  subject: string;
  email: string;
  emailVerified: boolean;
  name: string;
  role: "subscriber" | "creator" | "admin" | "guest";
  disabled: boolean;
  createdAt: string;
  lastLoginAt?: string;
};
export type ModelCall = {
  id: string;
  userId: string;
  userEmail: string;
  instanceId: string;
  model: string;
  modelUsed: string;
  credits: number;
  state: string;
  inputTokens: number;
  outputTokens: number;
  cost: number;
  rounds: number;
  errorCode?: string;
  settingsRevision: number;
  createdAt: string;
  settledAt?: string;
};
export type AuditEntry = {
  id: number;
  actorId?: string;
  action: string;
  subject: string;
  summary: unknown;
  createdAt: string;
};
export type SettingsDoc = {
  models: {
    allowed: { selector: string; label: string; thinking: string[]; credits: number }[];
    tiers: Record<string, string>;
  };
  allowance: {
    perUserConcurrency: number;
    globalConcurrency: number;
    maxToolRounds: number;
  };
  limits: {
    documentsPerInstance: number;
    assetBytesPerInstance: number;
    maxHtmlBytes: number;
    maxMediaBytes: number;
    maxAttachmentBytes: number;
  };
  publishing: { allowPinnedAfterWithdraw: boolean };
  plans: Plan[];
  demo: {
    enabled: boolean;
    ttlMinutes: number;
    maxActive: number;
    perAddressPerHour: number;
    credits: number;
    dailyBudget: number;
    models: string[];
  };
  storefront: { title: string; tagline: string; intro: string };
};
export type SettingsRevision = {
  revision: number;
  document: SettingsDoc;
  actorId?: string;
  activatedAt: string;
};
export type StripeEvent = {
  eventId: string;
  type: string;
  customerId: string;
  subscriptionId: string;
  state: string;
  attempts: number;
  errorCode?: string;
  receivedAt: string;
  processedAt?: string;
};

// The CSRF token comes from /api/session and is sent on every mutation.
let csrf = "";
export const setCSRF = (token?: string) => {
  csrf = token ?? "";
};

const baseQuery = fetchBaseQuery({
  baseUrl: "/",
  prepareHeaders: (headers, { type }) => {
    if (type === "mutation" && csrf) headers.set("X-CSRF-Token", csrf);
    return headers;
  },
});

export const api = createApi({
  reducerPath: "api",
  baseQuery,
  tagTypes: ["Session", "Catalog", "Account", "Instances", "Instance", "AdminReleases", "AdminUsers", "AdminUser", "AdminInstances", "Settings", "Billing", "Usage", "Audit", "Overview", "Tokens"],
  endpoints: (b) => ({
    session: b.query<Session, void>({
      query: () => "api/session",
      providesTags: ["Session"],
      async onQueryStarted(_, { queryFulfilled }) {
        try {
          setCSRF((await queryFulfilled).data.csrfToken);
        } catch {
          /* handled by the query state */
        }
      },
    }),
    // redirect, when present, is the identity provider's end-session URL.
    logout: b.mutation<{ ok: boolean; redirect?: string }, void>({
      query: () => ({ url: "auth/logout", method: "POST" }),
      invalidatesTags: ["Session", "Account", "Instances"],
    }),
    catalog: b.query<CatalogEntry[], void>({ query: () => "api/catalog", providesTags: ["Catalog"] }),
    artifact: b.query<ArtifactDetail, string>({ query: (id) => `api/artifacts/${id}`, providesTags: ["Catalog"] }),
    account: b.query<Account, void>({ query: () => "api/account", providesTags: ["Account"] }),
    checkout: b.mutation<{ url: string }, { plan: string; interval?: string }>({
      query: (body) => ({ url: "api/billing/checkout", method: "POST", body }),
    }),
    portal: b.mutation<{ url: string }, void>({ query: () => ({ url: "api/billing/portal", method: "POST" }) }),
    refreshBilling: b.mutation<{ queued: boolean }, string | void>({
      query: (sessionId) => ({ url: "api/billing/refresh", method: "POST", body: { sessionId: sessionId ?? "" } }),
    }),
    instances: b.query<Instance[], void>({ query: () => "api/instances", providesTags: ["Instances"] }),
    instance: b.query<Instance, string>({ query: (id) => `api/instances/${id}`, providesTags: ["Instance"] }),
    createInstance: b.mutation<Instance, { releaseId: string; title?: string }>({
      query: (body) => ({ url: "api/instances", method: "POST", body }),
      invalidatesTags: ["Instances"],
    }),
    renameInstance: b.mutation<unknown, { id: string; title: string }>({
      query: ({ id, title }) => ({ url: `api/instances/${id}`, method: "PATCH", body: { title } }),
      invalidatesTags: ["Instances", "Instance"],
    }),
    deleteInstance: b.mutation<unknown, string>({
      query: (id) => ({ url: `api/instances/${id}`, method: "DELETE" }),
      invalidatesTags: ["Instances"],
    }),
    launch: b.mutation<{ code: string; origin: string }, { id: string; challenge: string }>({
      query: ({ id, challenge }) => ({ url: `api/instances/${id}/launch`, method: "POST", body: { challenge } }),
    }),
    addMember: b.mutation<unknown, { id: string; email: string; role: string }>({
      query: ({ id, ...body }) => ({ url: `api/instances/${id}/members`, method: "POST", body }),
      invalidatesTags: ["Instance", "Instances"],
    }),
    removeMember: b.mutation<unknown, { id: string; userId: string }>({
      query: ({ id, userId }) => ({ url: `api/instances/${id}/members/${userId}`, method: "DELETE" }),
      invalidatesTags: ["Instance", "Instances"],
    }),
    upgrade: b.mutation<unknown, { id: string; releaseId: string }>({
      query: ({ id, releaseId }) => ({ url: `api/instances/${id}/upgrade`, method: "POST", body: { releaseId } }),
      invalidatesTags: ["Instance", "Instances"],
    }),
    // Administration.
    overview: b.query<any, void>({ query: () => "api/admin/overview", providesTags: ["Overview"] }),
    adminUsers: b.query<{ user: AdminUser; access: Decision }[], string>({
      query: (q) => `api/admin/users?q=${encodeURIComponent(q)}`,
      providesTags: ["AdminUsers"],
    }),
    adminUser: b.query<any, string>({ query: (id) => `api/admin/users/${id}`, providesTags: ["AdminUser"] }),
    patchUser: b.mutation<unknown, { id: string; role?: string; disabled?: boolean }>({
      query: ({ id, ...body }) => ({ url: `api/admin/users/${id}`, method: "PATCH", body }),
      invalidatesTags: ["AdminUsers", "AdminUser", "Overview"],
    }),
    revokeSessions: b.mutation<{ sessions: number; views: number }, string>({
      query: (id) => ({ url: `api/admin/users/${id}/revoke-sessions`, method: "POST" }),
      invalidatesTags: ["AdminUser"],
    }),
    grantAccess: b.mutation<unknown, { id: string; days: number; reason: string; plan?: string }>({
      query: ({ id, ...body }) => ({ url: `api/admin/users/${id}/access-grants`, method: "POST", body }),
      invalidatesTags: ["AdminUsers", "AdminUser"],
    }),
    revokeGrants: b.mutation<unknown, string>({
      query: (id) => ({ url: `api/admin/users/${id}/access-grants`, method: "DELETE" }),
      invalidatesTags: ["AdminUsers", "AdminUser"],
    }),
    adminInstances: b.query<Instance[], string>({
      query: (q) => `api/admin/instances?q=${encodeURIComponent(q)}`,
      providesTags: ["AdminInstances"],
    }),
    adminInstance: b.query<Instance, string>({ query: (id) => `api/admin/instances/${id}`, providesTags: ["AdminInstances"] }),
    setInstanceState: b.mutation<unknown, { id: string; action: "suspend" | "resume" }>({
      query: ({ id, action }) => ({ url: `api/admin/instances/${id}/${action}`, method: "POST" }),
      invalidatesTags: ["AdminInstances", "Instances", "Instance"],
    }),
    settings: b.query<{ active: SettingsRevision; history: SettingsRevision[]; providerModels: { selector: string; name: string; thinkingLevels: string[]; images: boolean }[]; secrets: Record<string, boolean> }, void>({
      query: () => "api/admin/settings",
      providesTags: ["Settings"],
    }),
    // If-Match names the revision the edit started from; the server answers
    // 412 settings_conflict when another revision was activated in between.
    saveSettings: b.mutation<SettingsRevision, { doc: SettingsDoc; revision: number }>({
      query: ({ doc, revision }) => ({ url: "api/admin/settings", method: "PATCH", body: doc, headers: { "If-Match": `"${revision}"` } }),
      invalidatesTags: ["Settings", "Account"],
    }),
    billing: b.query<any, string>({ query: (state) => `api/admin/billing?state=${state}`, providesTags: ["Billing"] }),
    reconcile: b.mutation<unknown, string>({
      query: (userId) => ({ url: "api/admin/billing/reconcile", method: "POST", body: { userId } }),
      invalidatesTags: ["Billing"],
    }),
    retryEvent: b.mutation<unknown, string>({
      query: (id) => ({ url: `api/admin/billing/events/${id}/retry`, method: "POST" }),
      invalidatesTags: ["Billing"],
    }),
    usage: b.query<{ summary: any; calls: ModelCall[] }, string>({
      query: (user) => `api/admin/usage?user=${encodeURIComponent(user)}&limit=200`,
      providesTags: ["Usage"],
    }),
    adjust: b.mutation<unknown, { userId: string; credits: number; reason: string }>({
      query: (body) => ({ url: "api/admin/usage/adjustments", method: "POST", body }),
      invalidatesTags: ["Usage", "AdminUser"],
    }),
    audit: b.query<AuditEntry[], string>({
      query: (subject) => `api/admin/audit?subject=${encodeURIComponent(subject)}`,
      providesTags: ["Audit"],
    }),
    tokens: b.query<Token[], void>({ query: () => "api/tokens", providesTags: ["Tokens"] }),
    createToken: b.mutation<{ token: string; info: Token }, { name: string; days: number; scopes: TokenScope[] }>({
      query: (body) => ({ url: "api/tokens", method: "POST", body }),
      invalidatesTags: ["Tokens"],
    }),
    revokeToken: b.mutation<unknown, string>({
      query: (id) => ({ url: `api/tokens/${id}`, method: "DELETE" }),
      invalidatesTags: ["Tokens"],
    }),
  }),
});

// errorMessage extracts the server's uniform error message.
export function errorMessage(e: unknown, fallback = "Something went wrong"): string {
  const d = (e as any)?.data;
  return d?.error?.message ?? (e as any)?.message ?? fallback;
}
export function errorCode(e: unknown): string | undefined {
  return (e as any)?.data?.error?.code;
}
