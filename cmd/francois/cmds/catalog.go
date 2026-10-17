package cmds

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog"
	"github.com/go-go-golems/francois/pkg/catalog/identity"
	"github.com/go-go-golems/francois/pkg/catalog/payments"
	"github.com/go-go-golems/francois/pkg/catalog/releases"
	"github.com/go-go-golems/francois/pkg/catalog/settings"
	cstore "github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/go-go-golems/francois/pkg/piworker"
	"github.com/go-go-golems/glazed/pkg/cli"
	glazed "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

type dbSettings struct {
	DatabaseURL     string `glazed:"database-url"`
	DatabaseURLFile string `glazed:"database-url-file"`
}

func dbFlags() []*fields.Definition {
	return []*fields.Definition{
		fields.New("database-url", fields.TypeSecret, fields.WithHelp("PostgreSQL URL for the hosted catalog (prefer --database-url-file)")),
		fields.New("database-url-file", fields.TypeString, fields.WithHelp("File containing the PostgreSQL URL")),
	}
}

// secret reads an inline value or a mounted file; file contents win.
func secret(value, file string) (string, error) {
	if file == "" {
		return strings.TrimSpace(value), nil
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return "", errors.Wrapf(err, "read secret file %s", file)
	}
	return strings.TrimSpace(string(b)), nil
}

func openCatalog(ctx context.Context, s dbSettings) (*cstore.Store, error) {
	u, err := secret(s.DatabaseURL, s.DatabaseURLFile)
	if err != nil {
		return nil, err
	}
	if u == "" {
		return nil, errors.New("a database URL is required (--database-url or --database-url-file)")
	}
	return cstore.Open(ctx, u)
}

type CatalogServeSettings struct {
	dbSettings
	Listen                string   `glazed:"listen"`
	PublicURL             string   `glazed:"public-url"`
	AppDomain             string   `glazed:"app-domain"`
	Sites                 []string `glazed:"site"`
	LegalContact          string   `glazed:"legal-contact"`
	TrustProxyHeaders     bool     `glazed:"trust-proxy-headers"`
	FontDir               string   `glazed:"font-dir"`
	MCP                   bool     `glazed:"mcp"`
	ReleaseDir            string   `glazed:"release-dir"`
	Migrate               bool     `glazed:"migrate"`
	OIDCIssuer            string   `glazed:"oidc-issuer"`
	OIDCClientID          string   `glazed:"oidc-client-id"`
	OIDCClientIDFile      string   `glazed:"oidc-client-id-file"`
	OIDCClientSecret      string   `glazed:"oidc-client-secret-file"`
	OIDCExtraScopes       []string `glazed:"oidc-extra-scopes"`
	EmailSigninEnabled    bool     `glazed:"email-signin-enabled"`
	ZitadelLoginTokenFile string   `glazed:"zitadel-login-token-file"`
	ZitadelOrganization   string   `glazed:"zitadel-organization"`
	SignupAllowed         bool     `glazed:"signup-allowed"`
	StripeKeyFile         string   `glazed:"stripe-secret-key-file"`
	StripeWebhookFiles    []string `glazed:"stripe-webhook-secret-files"`
	StripeLiveMode        bool     `glazed:"stripe-live-mode"`
	TrialAllowed          bool     `glazed:"trial-allowed"`
	GrantTTL              string   `glazed:"grant-ttl"`
	Backend               string   `glazed:"backend"`
	Node                  string   `glazed:"node"`
	Worker                string   `glazed:"worker"`
	PiAgentDir            string   `glazed:"pi-agent-dir"`
	WorkerDir             string   `glazed:"worker-dir"`
}

type CatalogServe struct{ *glazed.CommandDescription }

var _ glazed.BareCommand = (*CatalogServe)(nil)

func (c *CatalogServe) Run(ctx context.Context, parsed *values.Values) error {
	var s CatalogServeSettings
	if err := parsed.DecodeSectionInto(schema.DefaultSlug, &s); err != nil {
		return err
	}
	public, err := url.Parse(s.PublicURL)
	if err != nil {
		return errors.New("public-url must be an origin such as https://francois.example")
	}
	if err = identity.ValidateScopes(s.OIDCExtraScopes); err != nil {
		return err
	}
	db, err := openCatalog(ctx, s.dbSettings)
	if err != nil {
		return err
	}
	defer db.Close()
	if s.Migrate {
		applied, err := db.Migrate(ctx)
		if err != nil {
			return err
		}
		for _, a := range applied {
			log.Info().Str("migration", a).Msg("Applied migration")
		}
	}
	pending, err := db.Pending(ctx)
	if err != nil {
		return err
	}
	if len(pending) > 0 {
		return errors.Errorf("database has pending migrations %v; run francois catalog migrate", pending)
	}
	blobs, err := releases.OpenBlobs(s.ReleaseDir)
	if err != nil {
		return err
	}
	if s.OIDCClientIDFile != "" {
		if s.OIDCClientID, err = secret("", s.OIDCClientIDFile); err != nil {
			return err
		}
	}
	var clientSecret string
	if s.OIDCClientSecret != "" {
		if clientSecret, err = secret("", s.OIDCClientSecret); err != nil {
			return err
		}
	}
	// Every site signs in with the same client; each registers its own
	// callback URL on it.
	site := func(u *url.URL, appDomain string) catalog.Site {
		return catalog.Site{PublicURL: u, AppDomain: appDomain, OIDC: identity.New(identity.Config{Issuer: s.OIDCIssuer, ClientID: s.OIDCClientID,
			ClientSecret: clientSecret, RedirectURL: u.Scheme + "://" + u.Host + "/auth/callback", ExtraScopes: s.OIDCExtraScopes})}
	}
	sites := []catalog.Site{site(public, s.AppDomain)}
	for _, spec := range s.Sites {
		raw, appDomain, ok := strings.Cut(spec, "=")
		u, err := url.Parse(raw)
		if !ok || err != nil {
			return errors.Errorf("--site %q must look like https://portal.example=apps.example", spec)
		}
		sites = append(sites, site(u, appDomain))
	}
	cfg := catalog.Config{Sites: sites, TrialAllowed: s.TrialAllowed, SignupAllowed: s.SignupAllowed, LegalContact: s.LegalContact, TrustProxyHeaders: s.TrustProxyHeaders, FontDir: s.FontDir, MCP: s.MCP}
	if s.EmailSigninEnabled {
		token, err := secret("", s.ZitadelLoginTokenFile)
		if err != nil {
			return errors.Wrap(err, "read ZITADEL login service credential")
		}
		cfg.EmailLogin, err = identity.NewEmailLogin(s.OIDCIssuer, s.ZitadelOrganization, token)
		if err != nil {
			return err
		}
	}
	if err = cfg.Validate(); err != nil {
		return err
	}
	var pay *payments.Service
	if s.StripeKeyFile != "" {
		key, err := secret("", s.StripeKeyFile)
		if err != nil {
			return err
		}
		if strings.Contains(key, "_live_") != s.StripeLiveMode {
			return errors.New("Stripe key mode does not match --stripe-live-mode")
		}
		if s.StripeLiveMode && public.Scheme != "https" {
			return errors.New("live Stripe mode requires an HTTPS public URL")
		}
		hooks := []string{}
		for _, f := range s.StripeWebhookFiles {
			h, err := secret("", f)
			if err != nil {
				return err
			}
			if h != "" {
				hooks = append(hooks, h)
			}
		}
		if len(hooks) == 0 {
			return errors.New("Stripe requires at least one --stripe-webhook-secret-files entry; prices come from the plans in settings")
		}
		g, err := payments.NewStripe(key, hooks)
		if err != nil {
			return err
		}
		pay = payments.NewService(g, db, payments.Config{Livemode: s.StripeLiveMode, Deployment: "francois-" + public.Hostname()})
	}
	var backend piworker.Backend = &piworker.Fake{}
	if s.Backend == "pi" {
		worker, err := expand(s.Worker)
		if err != nil {
			return err
		}
		dir, err := expand(s.PiAgentDir)
		if err != nil {
			return err
		}
		cwd, err := expand(s.WorkerDir)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(cwd, 0o700); err != nil {
			return err
		}
		client, err := piworker.Start(ctx, s.Node, worker, dir, cwd, map[string]string{})
		if err != nil {
			return errors.Wrap(err, "start hosted Pi worker (use --backend fake for offline testing)")
		}
		backend = client
	}
	defer func() { _ = backend.Close() }()
	grantTTL, err := time.ParseDuration(s.GrantTTL)
	if err != nil || grantTTL < time.Minute || grantTTL > 12*time.Hour {
		return errors.New("grant-ttl must be a duration between 1m and 12h")
	}
	cfg.GrantTTL = grantTTL
	srv := catalog.New(ctx, cfg, db, &releases.Publisher{Store: db, Blobs: blobs}, pay, backend)
	return srv.Run(ctx, s.Listen)
}

type CatalogMigrate struct{ *glazed.CommandDescription }

var _ glazed.GlazeCommand = (*CatalogMigrate)(nil)

func (c *CatalogMigrate) RunIntoGlazeProcessor(ctx context.Context, p *values.Values, out middlewares.Processor) error {
	var s dbSettings
	if err := p.DecodeSectionInto(schema.DefaultSlug, &s); err != nil {
		return err
	}
	db, err := openCatalog(ctx, s)
	if err != nil {
		return err
	}
	defer db.Close()
	applied, err := db.Migrate(ctx)
	if err != nil {
		return err
	}
	for _, a := range applied {
		if err = out.AddRow(ctx, types.NewRow(types.MRP("migration", a), types.MRP("status", "applied"))); err != nil {
			return err
		}
	}
	return nil
}

type CatalogPublish struct{ *glazed.CommandDescription }

var _ glazed.GlazeCommand = (*CatalogPublish)(nil)

// RunIntoGlazeProcessor uses the same Publisher as the admin upload.
func (c *CatalogPublish) RunIntoGlazeProcessor(ctx context.Context, p *values.Values, out middlewares.Processor) error {
	var s struct {
		dbSettings
		ReleaseDir  string   `glazed:"release-dir"`
		ArtifactID  string   `glazed:"artifact-id"`
		Title       string   `glazed:"title"`
		Description string   `glazed:"description"`
		Tags        []string `glazed:"tags"`
		Version     string   `glazed:"version"`
		Changelog   string   `glazed:"changelog"`
		HTML        string   `glazed:"html"`
		Manifest    string   `glazed:"manifest"`
		Actor       string   `glazed:"actor-user-id"`
		Draft       bool     `glazed:"draft"`
		Free        bool     `glazed:"free-sample"`
	}
	if err := p.DecodeSectionInto(schema.DefaultSlug, &s); err != nil {
		return err
	}
	db, err := openCatalog(ctx, s.dbSettings)
	if err != nil {
		return err
	}
	defer db.Close()
	blobs, err := releases.OpenBlobs(s.ReleaseDir)
	if err != nil {
		return err
	}
	html, err := readHTML(s.HTML)
	if err != nil {
		return err
	}
	var manifest []byte
	if s.Manifest != "" {
		if manifest, err = os.ReadFile(s.Manifest); err != nil {
			return err
		}
	}
	if _, err = db.User(ctx, s.Actor); err != nil {
		return errors.Wrap(err, "actor-user-id must be an existing (admin) user")
	}
	if s.Title == "" {
		s.Title = s.ArtifactID
	}
	pub := &releases.Publisher{Store: db, Blobs: blobs}
	rel, checks, err := pub.CreateDraft(ctx, s.Actor, releases.Upload{ArtifactID: s.ArtifactID, Title: s.Title, Description: s.Description, Tags: s.Tags, Version: s.Version, Changelog: s.Changelog, HTML: html, Manifest: manifest, FreeSample: s.Free})
	for _, ch := range checks {
		log.Info().Str("level", ch.Level).Str("code", ch.Code).Msg(ch.Message)
	}
	if err != nil {
		return err
	}
	if !s.Draft {
		if rel, err = db.PublishRelease(ctx, rel.ID, s.Actor); err != nil {
			return err
		}
	}
	row := types.NewRow(types.MRP("release_id", rel.ID), types.MRP("artifact", rel.ArtifactID), types.MRP("version", rel.Version), types.MRP("state", rel.State))
	for _, f := range rel.Files {
		row.Set(f.Role+"_sha256", f.SHA256)
	}
	return out.AddRow(ctx, row)
}

type CatalogUsers struct{ *glazed.CommandDescription }

var _ glazed.GlazeCommand = (*CatalogUsers)(nil)

func (c *CatalogUsers) RunIntoGlazeProcessor(ctx context.Context, p *values.Values, out middlewares.Processor) error {
	var s struct {
		dbSettings
		Search string `glazed:"search"`
	}
	if err := p.DecodeSectionInto(schema.DefaultSlug, &s); err != nil {
		return err
	}
	db, err := openCatalog(ctx, s.dbSettings)
	if err != nil {
		return err
	}
	defer db.Close()
	users, err := db.Users(ctx, s.Search, 500)
	if err != nil {
		return err
	}
	for _, u := range users {
		if err = out.AddRow(ctx, types.NewRow(types.MRP("id", u.ID), types.MRP("email", u.Email), types.MRP("name", u.DisplayName), types.MRP("role", u.Role), types.MRP("disabled", u.Disabled), types.MRP("issuer", u.Issuer), types.MRP("subject", u.Subject))); err != nil {
			return err
		}
	}
	return nil
}

// CatalogUserAction implements operator recovery: promote (bootstrap admin by
// exact issuer/subject), disable, enable, revoke-sessions and grant-access.
type CatalogUserAction struct {
	*glazed.CommandDescription
	action string
}

var _ glazed.GlazeCommand = (*CatalogUserAction)(nil)

func (c *CatalogUserAction) RunIntoGlazeProcessor(ctx context.Context, p *values.Values, out middlewares.Processor) error {
	var s struct {
		dbSettings
		UserID  string `glazed:"user-id"`
		Issuer  string `glazed:"issuer"`
		Subject string `glazed:"subject"`
		Days    int    `glazed:"days"`
		Reason  string `glazed:"reason"`
		Plan    string `glazed:"plan"`
	}
	if err := p.DecodeSectionInto(schema.DefaultSlug, &s); err != nil {
		return err
	}
	db, err := openCatalog(ctx, s.dbSettings)
	if err != nil {
		return err
	}
	defer db.Close()
	var u cstore.User
	if s.UserID != "" {
		u, err = db.User(ctx, s.UserID)
	} else if s.Issuer != "" && s.Subject != "" {
		u, err = db.UserByIdentity(ctx, s.Issuer, s.Subject)
		if errors.Is(err, cstore.ErrNotFound) && c.action == "promote" {
			// Pre-provision the exact identity so its first login is admin.
			u, err = db.UpsertIdentity(ctx, cstore.Identity{Issuer: s.Issuer, Subject: s.Subject})
		}
	} else {
		return errors.New("--user-id or both --issuer and --subject are required")
	}
	if err != nil {
		return errors.Wrap(err, "find user")
	}
	switch c.action {
	case "promote":
		err = db.SetUserRole(ctx, u.ID, "admin")
	case "disable":
		err = db.SetUserDisabled(ctx, u.ID, true)
	case "enable":
		err = db.SetUserDisabled(ctx, u.ID, false)
	case "revoke-sessions":
		_, err = db.RevokeUserSessions(ctx, u.ID)
	case "grant-access":
		if s.Days < 1 || s.Reason == "" {
			return errors.New("--days and --reason are required")
		}
		_, err = db.GrantAccess(ctx, u.ID, u.ID, time.Now().Add(time.Duration(s.Days)*24*time.Hour), s.Reason, s.Plan)
	}
	if err != nil {
		return err
	}
	_ = db.Audit(ctx, "", "operator."+c.action, u.ID, map[string]any{"via": "cli"})
	return out.AddRow(ctx, types.NewRow(types.MRP("user_id", u.ID), types.MRP("action", c.action), types.MRP("status", "ok")))
}

type CatalogSettings struct{ *glazed.CommandDescription }

var _ glazed.GlazeCommand = (*CatalogSettings)(nil)

// RunIntoGlazeProcessor shows the active settings or activates a validated
// JSON settings file.
func (c *CatalogSettings) RunIntoGlazeProcessor(ctx context.Context, p *values.Values, out middlewares.Processor) error {
	var s struct {
		dbSettings
		File string `glazed:"file"`
	}
	if err := p.DecodeSectionInto(schema.DefaultSlug, &s); err != nil {
		return err
	}
	db, err := openCatalog(ctx, s.dbSettings)
	if err != nil {
		return err
	}
	defer db.Close()
	rev, err := db.ActiveSettings(ctx)
	if err != nil {
		return err
	}
	if s.File != "" {
		raw, err := os.ReadFile(s.File)
		if err != nil {
			return err
		}
		doc, err := settings.Parse(raw)
		if err != nil {
			return err
		}
		if rev, err = db.SaveSettings(ctx, "", doc); err != nil {
			return err
		}
	}
	raw, _ := json.Marshal(rev.Document)
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	return out.AddRow(ctx, types.NewRow(types.MRP("revision", rev.Revision), types.MRP("settings", doc)))
}

func registerCatalog(root *cobra.Command) error {
	group := &cobra.Command{Use: "catalog", Short: "Hosted subscriber platform: portal, admin, instances and billing"}
	db := &cobra.Command{Use: "db", Short: "Direct database operations for operators (bootstrap and break-glass; prefer the API commands)"}
	users := &cobra.Command{Use: "users", Short: "Operator account management directly in the database"}
	withDB := func(extra ...*fields.Definition) glazed.CommandDescriptionOption {
		return glazed.WithFlags(append(dbFlags(), extra...)...)
	}
	cwd, _ := os.Getwd()
	serve := &CatalogServe{glazed.NewCommandDescription("serve", glazed.WithShort("Serve the subscriber portal, admin UI and isolated instance origins"),
		glazed.WithLong("Serves the trusted portal at --public-url and instance origins at i-<id>.<app-domain> from one listener. Each --site adds another portal origin and application domain for the same catalog; register its /auth/callback and / on the OIDC client. Secrets are read from files; no subscriber home directory is used. --backend fake runs without Node or Pi credentials."),
		withDB(
			fields.New("listen", fields.TypeString, fields.WithDefault("127.0.0.1:8090"), fields.WithHelp("Listen address")),
			fields.New("public-url", fields.TypeString, fields.WithDefault("http://localhost:8090"), fields.WithHelp("Trusted portal origin")),
			fields.New("app-domain", fields.TypeString, fields.WithDefault("apps.localhost"), fields.WithHelp("Domain for per-instance application origins")),
			fields.New("legal-contact", fields.TypeString, fields.WithHelp("Contact address shown on /privacy and /terms")),
			fields.New("trust-proxy-headers", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Take visitor addresses from X-Real-Ip/X-Forwarded-For (only behind a trusted ingress); used for demo rate limits")),
			fields.New("mcp", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Serve MCP tools at /mcp for remote agents with a personal API token")),
			fields.New("font-dir", fields.TypeString, fields.WithHelp("Private directory of licensed WOFF2 web fonts (ABC Areal); never commit these files")),
			fields.New("site", fields.TypeStringList, fields.WithHelp("Additional site as <portal origin>=<app domain>, e.g. https://slop.boutique=slop.boutique; the app domain may equal the portal hostname")),
			fields.New("release-dir", fields.TypeString, fields.WithDefault("./var/catalog/releases"), fields.WithHelp("Private immutable release file directory")),
			fields.New("migrate", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Apply pending migrations before serving")),
			fields.New("email-signin-enabled", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Enable ZITADEL email-only sign-in for existing supporter accounts")),
			fields.New("zitadel-login-token-file", fields.TypeString, fields.WithHelp("Secret file containing the dedicated ZITADEL login service token")),
			fields.New("zitadel-organization", fields.TypeString, fields.WithHelp("ZITADEL organization whose users may sign in by email")),
			fields.New("oidc-issuer", fields.TypeString, fields.WithHelp("OIDC issuer URL (ZITADEL)")),
			fields.New("oidc-client-id", fields.TypeString, fields.WithHelp("OIDC web client ID (PKCE)")),
			fields.New("oidc-client-id-file", fields.TypeString, fields.WithHelp("File containing the OIDC client ID (written by provisioning)")),
			fields.New("oidc-client-secret-file", fields.TypeString, fields.WithHelp("Optional OIDC client secret file")),
			fields.New("oidc-extra-scopes", fields.TypeStringList, fields.WithHelp("Additional OIDC scopes, e.g. urn:zitadel:iam:org:id:<id> to select the ZITADEL organization")),
			fields.New("signup-allowed", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Offer the identity provider's registration form; leave off for invite-only deployments")),
			fields.New("stripe-secret-key-file", fields.TypeString, fields.WithHelp("Stripe secret key file; billing is disabled when empty")),
			fields.New("stripe-webhook-secret-files", fields.TypeStringList, fields.WithHelp("Current and previous webhook signing secret files")),
			fields.New("stripe-live-mode", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Expect live-mode keys and events")),
			fields.New("grant-ttl", fields.TypeString, fields.WithDefault("30m"), fields.WithHelp("Lifetime of instance runtime grants (renewed through the portal)")),
			fields.New("trial-allowed", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Grant access while a subscription is trialing")),
			fields.New("backend", fields.TypeChoice, fields.WithChoices("pi", "fake"), fields.WithDefault("fake"), fields.WithHelp("Operator Pi model worker or deterministic fake")),
			fields.New("node", fields.TypeString, fields.WithDefault("node"), fields.WithHelp("Node executable")),
			fields.New("worker", fields.TypeString, fields.WithDefault(filepath.Join(cwd, "worker/dist/main.js")), fields.WithHelp("Built Pi worker entrypoint")),
			fields.New("pi-agent-dir", fields.TypeString, fields.WithDefault("/etc/francois/pi"), fields.WithHelp("Operator-controlled Pi provider configuration directory")),
			fields.New("worker-dir", fields.TypeString, fields.WithDefault("./var/catalog/worker"), fields.WithHelp("Model worker working directory")),
		))}
	migrate := &CatalogMigrate{glazed.NewCommandDescription("migrate", glazed.WithShort("Apply catalog PostgreSQL migrations"), withDB())}
	publish := &CatalogPublish{glazed.NewCommandDescription("publish", glazed.WithShort("Create (and publish) a release with the admin publication service"),
		withDB(
			fields.New("release-dir", fields.TypeString, fields.WithDefault("./var/catalog/releases")),
			fields.New("artifact-id", fields.TypeString, fields.WithRequired(true)),
			fields.New("title", fields.TypeString),
			fields.New("description", fields.TypeString),
			fields.New("tags", fields.TypeStringList),
			fields.New("version", fields.TypeString, fields.WithRequired(true)),
			fields.New("changelog", fields.TypeString),
			fields.New("html", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Authored HTML file")),
			fields.New("manifest", fields.TypeString, fields.WithHelp("Capability manifest JSON file")),
			fields.New("actor-user-id", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Admin user recorded as author")),
			fields.New("draft", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Stop at draft; publish later from the admin UI")),
			fields.New("free-sample", fields.TypeBool, fields.WithDefault(false)),
		))}
	list := &CatalogUsers{glazed.NewCommandDescription("list", glazed.WithShort("List accounts"), withDB(fields.New("search", fields.TypeString)))}
	settingsCmd := &CatalogSettings{glazed.NewCommandDescription("settings", glazed.WithShort("Show or activate validated platform settings"), withDB(fields.New("file", fields.TypeString, fields.WithHelp("JSON settings document to validate and activate"))))}
	for _, cmd := range []glazed.Command{serve, migrate} {
		c, err := cli.BuildCobraCommandFromCommand(cmd)
		if err != nil {
			return err
		}
		group.AddCommand(c)
	}
	for _, cmd := range []glazed.Command{publish, settingsCmd} {
		c, err := cli.BuildCobraCommandFromCommand(cmd)
		if err != nil {
			return err
		}
		db.AddCommand(c)
	}
	userFlags := withDB(fields.New("user-id", fields.TypeString), fields.New("issuer", fields.TypeString), fields.New("subject", fields.TypeString), fields.New("days", fields.TypeInteger, fields.WithDefault(0)), fields.New("reason", fields.TypeString), fields.New("plan", fields.TypeString, fields.WithHelp("grant-access: plan key (default: the highest plan)")))
	actions := map[string]string{"promote": "Make an exact issuer/subject (or user) an administrator", "disable": "Disable hosted access and revoke runtime grants", "enable": "Re-enable an account", "revoke-sessions": "Revoke portal sessions and runtime grants", "grant-access": "Grant complimentary hosted access for --days"}
	c, err := cli.BuildCobraCommandFromCommand(list)
	if err != nil {
		return err
	}
	users.AddCommand(c)
	for name, short := range actions {
		c, err := cli.BuildCobraCommandFromCommand(&CatalogUserAction{glazed.NewCommandDescription(name, glazed.WithShort(short), userFlags), name})
		if err != nil {
			return err
		}
		users.AddCommand(c)
	}
	db.AddCommand(users)
	group.AddCommand(db)
	if err := registerAPI(group); err != nil {
		return err
	}
	root.AddCommand(group)
	return nil
}
