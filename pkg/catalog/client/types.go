package client

import (
	"encoding/json"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/billing"
	"github.com/go-go-golems/francois/pkg/catalog/settings"
)

// The wire types mirror the server's JSON. Pure packages (settings, billing)
// are reused; nested values the client passes through without interpreting
// stay json.RawMessage. Contract tests decode real responses strictly
// (WithStrictDecoding) so a server field missing here fails a test.

// Me is GET /api/me.
type Me struct {
	User    MeUser   `json:"user"`
	Token   *Token   `json:"token"`
	Creator *Creator `json:"creator"`
}

type MeUser struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Disabled bool   `json:"disabled"`
}

// Token is a personal API token's metadata (never its secret).
type Token struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	RevokedAt  *time.Time `json:"revokedAt,omitempty"`
}

// Allows reports whether the token carries scope; admin:write implies admin:read.
func (t *Token) Allows(scope string) bool {
	if t == nil {
		return false
	}
	for _, s := range t.Scopes {
		if s == scope || (scope == "admin:read" && s == "admin:write") {
			return true
		}
	}
	return false
}

type User struct {
	ID            string     `json:"id"`
	Issuer        string     `json:"issuer"`
	Subject       string     `json:"subject"`
	Email         string     `json:"email"`
	EmailVerified bool       `json:"emailVerified"`
	Name          string     `json:"name"`
	Role          string     `json:"role"`
	Disabled      bool       `json:"disabled"`
	CreatedAt     time.Time  `json:"createdAt"`
	LastLoginAt   *time.Time `json:"lastLoginAt,omitempty"`
}

type Page struct {
	Body    string   `json:"body"`
	Cover   string   `json:"cover,omitempty"`
	Gallery []string `json:"gallery"`
}

type Artifact struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Tags         []string  `json:"tags"`
	Visibility   string    `json:"visibility"`
	Page         Page      `json:"page"`
	PageDraft    *Page     `json:"pageDraft,omitempty"`
	Featured     bool      `json:"featured"`
	SortOrder    int       `json:"sortOrder"`
	RequiredPlan string    `json:"requiredPlan"`
	DemoEnabled  bool      `json:"demoEnabled"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// ArtifactRow is a studio list row.
type ArtifactRow struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Tags          []string  `json:"tags"`
	Visibility    string    `json:"visibility"`
	Featured      bool      `json:"featured"`
	SortOrder     int       `json:"sortOrder"`
	RequiredPlan  string    `json:"requiredPlan"`
	DemoEnabled   bool      `json:"demoEnabled"`
	Cover         string    `json:"cover"`
	HasDraft      bool      `json:"hasDraft"`
	Creators      []string  `json:"creators"`
	Releases      int       `json:"releases"`
	DraftReleases int       `json:"draftReleases"`
	LatestVersion string    `json:"latestVersion"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// ArtifactDetail is GET /api/studio/artifacts/{id}.
type ArtifactDetail struct {
	Artifact Artifact  `json:"artifact"`
	Creators []string  `json:"creators"`
	Releases []Release `json:"releases"`
}

// Patch inputs use pointers: nil leaves a field unchanged, a pointer to an
// empty value clears it (the server distinguishes absent from empty).

// String, Strings and Int make pointers for patch inputs.
func String(s string) *string      { return &s }
func Strings(v []string) *[]string { return &v }
func Int(n int) *int               { return &n }

// ArtifactPatch changes only the fields that are set.
type ArtifactPatch struct {
	Title        *string   `json:"title,omitempty"`
	Description  *string   `json:"description,omitempty"`
	Tags         *[]string `json:"tags,omitempty"`
	Visibility   *string   `json:"visibility,omitempty"`
	Featured     *bool     `json:"featured,omitempty"`
	SortOrder    *int      `json:"sortOrder,omitempty"`
	RequiredPlan *string   `json:"requiredPlan,omitempty"`
	DemoEnabled  *bool     `json:"demoEnabled,omitempty"`
	Creators     *[]string `json:"creators,omitempty"`
}

type ReleaseFile struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Name      string `json:"name"`
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
}

type Attachment struct {
	ID        string     `json:"id"`
	ReleaseID string     `json:"releaseId,omitempty"`
	Name      string     `json:"name"`
	Label     string     `json:"label"`
	MediaType string     `json:"mediaType"`
	Size      int64      `json:"size"`
	SHA256    string     `json:"sha256"`
	Position  int        `json:"position"`
	Href      string     `json:"href,omitempty"`
	CreatedBy string     `json:"createdBy,omitempty"`
	CreatedAt *time.Time `json:"createdAt,omitempty"`
}

// Release is a release as the studio and public APIs return it; studio-only
// fields are empty in public responses.
type Release struct {
	ID              string          `json:"id"`
	ArtifactID      string          `json:"artifactId"`
	Version         string          `json:"version"`
	Changelog       string          `json:"changelog"`
	State           string          `json:"state"`
	PublishedAt     *time.Time      `json:"publishedAt"`
	Capabilities    []string        `json:"capabilities"`
	Files           []ReleaseFile   `json:"files"`
	Attachments     []Attachment    `json:"attachments"`
	DownloadAccess  string          `json:"downloadAccess"`
	CreatedAt       *time.Time      `json:"createdAt,omitempty"`
	WithdrawnAt     *time.Time      `json:"withdrawnAt,omitempty"`
	FreeSample      bool            `json:"freeSample,omitempty"`
	PinnedInstances int             `json:"pinnedInstances,omitempty"`
	Manifest        json.RawMessage `json:"manifest,omitempty"`
	Checks          []Check         `json:"checks,omitempty"`
}

// CheckResult is POST /api/studio/releases/check.
type CheckResult struct {
	Checks   []Check         `json:"checks"`
	Blocking bool            `json:"blocking"`
	Manifest json.RawMessage `json:"manifest"`
	Size     int64           `json:"size"`
}

type Media struct {
	ID         string    `json:"id"`
	SHA256     string    `json:"sha256"`
	MediaType  string    `json:"mediaType"`
	Size       int64     `json:"size"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	Alt        string    `json:"alt"`
	UploadedBy string    `json:"uploadedBy"`
	CreatedAt  time.Time `json:"createdAt"`
	Poster     string    `json:"poster"`
}

type Post struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Summary     string     `json:"summary"`
	Body        string     `json:"body"`
	Cover       string     `json:"cover,omitempty"`
	AuthorID    string     `json:"authorId,omitempty"`
	Tags        []string   `json:"tags"`
	State       string     `json:"state"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
	CreatedBy   string     `json:"createdBy"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// PostInput creates or edits a journal post; nil fields are left unchanged.
type PostInput struct {
	Slug     *string   `json:"slug,omitempty"`
	Title    *string   `json:"title,omitempty"`
	Summary  *string   `json:"summary,omitempty"`
	Body     *string   `json:"body,omitempty"`
	Cover    *string   `json:"cover,omitempty"`
	AuthorID *string   `json:"authorId,omitempty"`
	Tags     *[]string `json:"tags,omitempty"`
}

type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type Creator struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	Bio       string    `json:"bio"`
	Avatar    string    `json:"avatar,omitempty"`
	Links     []Link    `json:"links"`
	UserID    string    `json:"userId,omitempty"`
	SortOrder int       `json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// CreatorInput creates or edits a creator profile; nil fields are left
// unchanged. userId and sortOrder are admin-only.
type CreatorInput struct {
	Slug      *string `json:"slug,omitempty"`
	Name      *string `json:"name,omitempty"`
	Bio       *string `json:"bio,omitempty"`
	Avatar    *string `json:"avatar,omitempty"`
	Links     *[]Link `json:"links,omitempty"`
	UserID    *string `json:"userId,omitempty"`
	SortOrder *int    `json:"sortOrder,omitempty"`
}

// PlanName is GET /api/studio/plans.
type PlanName struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Rank   int    `json:"rank"`
	Public bool   `json:"public"`
}

// StudioMe is GET /api/studio/me.
type StudioMe struct {
	User    struct{ ID, Name, Role string } `json:"user"`
	Creator *Creator                        `json:"creator"`
	Limits  struct {
		MaxMediaBytes      int64 `json:"maxMediaBytes"`
		MaxAttachmentBytes int64 `json:"maxAttachmentBytes"`
	} `json:"limits"`
}

type Member struct {
	UserID    string    `json:"userId"`
	Name      string    `json:"name"`
	Email     string    `json:"email,omitempty"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

// Instance is a private copy of a release; detail responses add the fields
// after Members.
type Instance struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Kind        string     `json:"kind"`
	State       string     `json:"state"`
	Role        string     `json:"role"`
	ArtifactID  string     `json:"artifactId"`
	Version     string     `json:"version"`
	ReleaseID   string     `json:"releaseId"`
	OwnerName   string     `json:"ownerName"`
	OwnerUserID string     `json:"ownerUserId"`
	Members     int        `json:"members"`
	Origin      string     `json:"origin"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`

	ActiveViews    int             `json:"activeViews,omitempty"`
	MembersList    []Member        `json:"membersList,omitempty"`
	Usage          json.RawMessage `json:"usage,omitempty"`
	RecentFailures json.RawMessage `json:"recentFailures,omitempty"`
	Audit          []AuditEntry    `json:"audit,omitempty"`
	Release        *Release        `json:"release,omitempty"`
	Upgrades       []Release       `json:"upgrades,omitempty"`
	LaunchBlocked  string          `json:"launchBlocked,omitempty"`
}

type AccessGrant struct {
	ID        string     `json:"id"`
	UserID    string     `json:"userId"`
	Until     time.Time  `json:"until"`
	Reason    string     `json:"reason"`
	GrantedBy string     `json:"grantedBy"`
	CreatedAt time.Time  `json:"createdAt"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
	Plan      string     `json:"plan"`
}

type AuditEntry struct {
	ID        int64           `json:"id"`
	ActorID   *string         `json:"actorId"`
	TokenID   *string         `json:"tokenId,omitempty"`
	Action    string          `json:"action"`
	Subject   string          `json:"subject"`
	Summary   json.RawMessage `json:"summary"`
	CreatedAt time.Time       `json:"createdAt"`
}

// UserRow is a row of GET /api/admin/users.
type UserRow struct {
	User    User             `json:"user"`
	Access  billing.Decision `json:"access"`
	Plan    string           `json:"plan"`
	PlanKey string           `json:"planKey"`
	Source  string           `json:"source"`
}

// Customer is GET /api/admin/users/{id}: everything an operator needs about
// one account.
type Customer struct {
	User             User                         `json:"user"`
	Access           billing.Decision             `json:"access"`
	Entitlement      json.RawMessage              `json:"entitlement"`
	Plans            []struct{ Key, Name string } `json:"plans"`
	Subscriptions    []billing.Subscription       `json:"subscriptions"`
	Complimentary    []AccessGrant                `json:"complimentary"`
	Instances        []Instance                   `json:"instances"`
	Sessions         int                          `json:"sessions"`
	StripeCustomerID string                       `json:"stripeCustomerId"`
	Allowance        json.RawMessage              `json:"allowance"`
	Calls            json.RawMessage              `json:"calls"`
	Audit            []AuditEntry                 `json:"audit"`
}

// UserPatch changes a user's role or disabled flag.
type UserPatch struct {
	Role     *string `json:"role,omitempty"`
	Disabled *bool   `json:"disabled,omitempty"`
}

// GrantRequest is a complimentary access grant; Plan "" means the highest plan.
type GrantRequest struct {
	Days   int    `json:"days"`
	Reason string `json:"reason"`
	Plan   string `json:"plan,omitempty"`
}

// Revoked is POST /api/admin/users/{id}/revoke-sessions.
type Revoked struct {
	Sessions int64 `json:"sessions"`
	Views    int   `json:"views"`
	Tokens   int64 `json:"tokens"`
}

// SubscriptionRow is a row of GET /api/admin/subscriptions.
type SubscriptionRow struct {
	Subscription billing.Subscription `json:"subscription"`
	UserID       string               `json:"userId"`
	Email        string               `json:"email"`
	Plan         string               `json:"plan"`
}

// SettingsRevision is one activated settings document.
type SettingsRevision struct {
	Revision    int64             `json:"revision"`
	Document    settings.Document `json:"document"`
	ActorID     *string           `json:"actorId"`
	ActivatedAt time.Time         `json:"activatedAt"`
}

// Settings is GET /api/admin/settings.
type Settings struct {
	Active         SettingsRevision   `json:"active"`
	History        []SettingsRevision `json:"history"`
	ProviderModels json.RawMessage    `json:"providerModels"`
	Secrets        map[string]bool    `json:"secrets"`
	Defaults       settings.Document  `json:"defaults"`
}

// UserQuery filters GET /api/admin/users.
type UserQuery struct {
	Q, Role, Plan string
	Limit         int
}

// AuditQuery filters GET /api/admin/audit.
type AuditQuery struct {
	Subject, Actor, Action string
	Before                 int64
	Limit                  int
}

// Usage is GET /api/admin/usage.
type Usage struct {
	Summary json.RawMessage `json:"summary"`
	Calls   json.RawMessage `json:"calls"`
}

// ArtifactCreate is POST /api/studio/artifacts.
type ArtifactCreate struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}
