package cmds

import (
	"context"
	"os"

	"github.com/go-go-golems/francois/pkg/catalog/client"
	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// registerAPI adds the API command groups under `francois catalog`.
func registerAPI(catalog *cobra.Command) error {
	builders := []func() (*cobra.Command, error){
		whoamiCommand, artifactCommands, releaseCommands, attachmentCommands, mediaCommands, postCommands, creatorCommands,
		userCommands, subscriptionCommands, instanceCommands, planCommands, settingsCommands, billingCommands, usageCommands,
		auditCommands, accountCommands,
	}
	for _, b := range builders {
		g, err := b()
		if err != nil {
			return err
		}
		catalog.AddCommand(g)
	}
	return nil
}

func whoamiCommand() (*cobra.Command, error) {
	return single(def{name: "whoami", short: "Show the user and scopes the token acts with",
		run: func(ctx context.Context, c *client.Client, _ in) (any, error) { return c.Me(ctx) }})
}

func artifactCommands() (*cobra.Command, error) {
	return group("artifacts", "Artifacts and their public pages (scope studio)",
		def{name: "list", short: "List artifacts you may edit", run: func(ctx context.Context, c *client.Client, _ in) (any, error) { return c.Studio.Artifacts(ctx) }},
		def{name: "show", short: "Show an artifact with its releases", flags: argID("artifact ID"),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return c.Studio.Artifact(ctx, a.S("id"))
			}},
		def{name: "create", short: "Create an artifact", flags: append(argID("new artifact ID (lowercase, digits, dashes)"), required("title", "Title"), flag("description", "Short description")),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return c.Studio.CreateArtifact(ctx, client.ArtifactCreate{ID: a.S("id"), Title: a.S("title"), Description: a.S("description")})
			}},
		def{name: "update", short: "Change an artifact's metadata (only the flags given)",
			flags: append(argID("artifact ID"), flag("title", "Title"), flag("description", "Description"), list("tags", "Tags (replaces all)"),
				flag("visibility", "listed or hidden"), flag("featured", "true or false"), flag("sort-order", "Position on the storefront"),
				flag("required-plan", "Plan key needed to run it"), flag("demo-enabled", "true or false"), list("creators", "Creator profile IDs (replaces all)")),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				p := client.ArtifactPatch{Title: a.optS("title"), Description: a.optS("description"), Tags: a.L("tags"), Visibility: a.optS("visibility"),
					RequiredPlan: a.optS("required-plan"), Creators: a.L("creators")}
				var err error
				if p.Featured, err = a.optB("featured"); err != nil {
					return nil, err
				}
				if p.DemoEnabled, err = a.optB("demo-enabled"); err != nil {
					return nil, err
				}
				if p.SortOrder, err = a.optI("sort-order"); err != nil {
					return nil, err
				}
				return c.Studio.PatchArtifact(ctx, a.S("id"), p)
			}},
		def{name: "page-set", short: "Save a draft of the artifact's public page",
			flags: append(argID("artifact ID"), required("body-file", "Markdown file for the page body"), flag("cover", "Cover media ID"), list("gallery", "Gallery media IDs")),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				body, err := os.ReadFile(a.S("body-file")) // #nosec G304 -- the operator names the file
				if err != nil {
					return nil, err
				}
				gallery := a.L("gallery")
				if gallery == nil {
					gallery = []string{}
				}
				return nil, c.Studio.SaveDraft(ctx, a.S("id"), client.Page{Body: string(body), Cover: a.S("cover"), Gallery: gallery})
			}},
		def{name: "page-publish", short: "Publish the page draft", flags: argID("artifact ID"),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return nil, c.Studio.PublishDraft(ctx, a.S("id"))
			}},
		def{name: "page-discard", short: "Discard the page draft", flags: argID("artifact ID"), destructive: true,
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return nil, c.Studio.DiscardDraft(ctx, a.S("id"))
			}},
	)
}

func argID(help string) []*fieldsDef { return []*fieldsDef{arg("id", help)} }

func pushCommand() (*cobra.Command, error) {
	return cli.BuildCobraCommandFromCommand(newCatalogPush())
}

func releaseCommands() (*cobra.Command, error) {
	g, err := group("releases", "Releases (scope studio)",
		def{name: "list", short: "List releases", flags: []*fieldsDef{flag("artifact", "Artifact ID (required for creators)")},
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return c.Studio.Releases(ctx, a.S("artifact"))
			}},
		def{name: "publish", short: "Publish a draft release", flags: argID("release ID"),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return c.Studio.PublishRelease(ctx, a.S("id"))
			}},
		def{name: "withdraw", short: "Withdraw a published release", destructive: true,
			flags: append(argID("release ID"), boolean("suspend-pinned", "Also suspend workspaces pinned to it")),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				n, err := c.Studio.WithdrawRelease(ctx, a.S("id"), a.B("suspend-pinned"))
				return map[string]any{"withdrawn": err == nil, "suspendedInstances": n}, err
			}},
		def{name: "preview", short: "Open a preview instance of a draft", flags: argID("release ID"),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return c.Studio.PreviewRelease(ctx, a.S("id"))
			}},
		def{name: "delete", short: "Delete an unused draft release", flags: argID("release ID"), destructive: true,
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return nil, c.Studio.DeleteRelease(ctx, a.S("id"))
			}},
	)
	if err != nil {
		return nil, err
	}
	push, err := pushCommand()
	if err != nil {
		return nil, err
	}
	g.AddCommand(push)
	return g, nil
}

func attachmentCommands() (*cobra.Command, error) {
	return group("attachments", "Files attached to releases (scope studio)",
		def{name: "add", short: "Attach a file to a release",
			flags: []*fieldsDef{arg("release", "release ID"), arg("file", "local file"), flag("label", "Label shown to subscribers")},
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return c.Studio.AddAttachmentFile(ctx, a.S("release"), a.S("file"), a.S("label"))
			}},
		def{name: "update", short: "Change an attachment's label or position", flags: append(argID("attachment ID"), flag("label", "Label"), flag("position", "Position")),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				pos, err := a.optI("position")
				if err != nil {
					return nil, err
				}
				return c.Studio.PatchAttachment(ctx, a.S("id"), a.optS("label"), pos)
			}},
		def{name: "delete", short: "Delete an attachment", flags: argID("attachment ID"), destructive: true,
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return nil, c.Studio.DeleteAttachment(ctx, a.S("id"))
			}},
		def{name: "download", short: "Download an attachment", flags: append(argID("attachment ID"), required("output", "File to write")),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				f, err := os.Create(a.S("output")) // #nosec G304 -- the operator names the output file
				if err != nil {
					return nil, err
				}
				defer func() { _ = f.Close() }()
				n, err := c.Studio.DownloadAttachment(ctx, a.S("id"), f)
				return map[string]any{"file": a.S("output"), "bytes": n}, err
			}},
	)
}

func mediaCommands() (*cobra.Command, error) {
	return group("media", "Images and videos for pages and posts (scope studio)",
		def{name: "list", short: "List the media library", run: func(ctx context.Context, c *client.Client, _ in) (any, error) { return c.Studio.Media(ctx) }},
		def{name: "upload", short: "Upload an image or video", flags: []*fieldsDef{arg("file", "local image or video"), flag("alt", "Alt text")},
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return c.Studio.UploadMediaFile(ctx, a.S("file"), a.S("alt"))
			}},
		def{name: "update", short: "Set alt text or a video's poster image", flags: append(argID("media ID"), flag("alt", "Alt text"), flag("poster", "Poster image media ID (videos)")),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return nil, c.Studio.PatchMedia(ctx, a.S("id"), a.optS("alt"), a.optS("poster"))
			}},
		def{name: "delete", short: "Delete unused media", flags: argID("media ID"), destructive: true,
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return nil, c.Studio.DeleteMedia(ctx, a.S("id"))
			}},
	)
}

func postInput(a in) (client.PostInput, error) {
	p := client.PostInput{Slug: a.S("slug"), Title: a.S("title"), Summary: a.S("summary"), Cover: a.S("cover"), AuthorID: a.S("author"), Tags: a.L("tags")}
	if f := a.S("body-file"); f != "" {
		body, err := os.ReadFile(f) // #nosec G304 -- the operator names the file
		if err != nil {
			return p, err
		}
		p.Body = string(body)
	}
	return p, nil
}

func postFlags() []*fieldsDef {
	return []*fieldsDef{flag("summary", "Summary"), flag("body-file", "Markdown file for the body"), flag("cover", "Cover media ID"), flag("author", "Author creator ID"), list("tags", "Tags")}
}

func postCommands() (*cobra.Command, error) {
	return group("posts", "Journal posts (scope studio)",
		def{name: "list", short: "List posts you may edit", run: func(ctx context.Context, c *client.Client, _ in) (any, error) { return c.Studio.Posts(ctx) }},
		def{name: "show", short: "Show a post", flags: argID("post ID"),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) { return c.Studio.Post(ctx, a.S("id")) }},
		def{name: "create", short: "Create a draft post", flags: append([]*fieldsDef{required("slug", "URL slug"), required("title", "Title")}, postFlags()...),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				p, err := postInput(a)
				if err != nil {
					return nil, err
				}
				return c.Studio.CreatePost(ctx, p)
			}},
		def{name: "update", short: "Edit a post (only the flags given)", flags: append(append(argID("post ID"), flag("slug", "URL slug"), flag("title", "Title")), postFlags()...),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				p, err := postInput(a)
				if err != nil {
					return nil, err
				}
				return c.Studio.PatchPost(ctx, a.S("id"), p)
			}},
		def{name: "publish", short: "Publish a post", flags: argID("post ID"),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return nil, c.Studio.SetPostPublished(ctx, a.S("id"), true)
			}},
		def{name: "unpublish", short: "Unpublish a post", flags: argID("post ID"),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return nil, c.Studio.SetPostPublished(ctx, a.S("id"), false)
			}},
		def{name: "delete", short: "Delete a post", flags: argID("post ID"), destructive: true,
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return nil, c.Studio.DeletePost(ctx, a.S("id"))
			}},
	)
}

func creatorInput(a in) (client.CreatorInput, error) {
	sort, err := a.optI("sort-order")
	return client.CreatorInput{Slug: a.S("slug"), Name: a.S("name"), Bio: a.S("bio"), Avatar: a.S("avatar"), UserID: a.S("user-id"), SortOrder: sort}, err
}

func creatorFlags() []*fieldsDef {
	return []*fieldsDef{flag("bio", "Biography (Markdown)"), flag("avatar", "Avatar media ID"), flag("user-id", "Linked account (admin only)"), flag("sort-order", "Order on the storefront (admin only)")}
}

func creatorCommands() (*cobra.Command, error) {
	return group("creators", "Creator profiles (scope studio; create and delete are admin-only)",
		def{name: "list", short: "List creators", run: func(ctx context.Context, c *client.Client, _ in) (any, error) { return c.Studio.Creators(ctx) }},
		def{name: "create", short: "Create a creator profile", flags: append([]*fieldsDef{required("slug", "URL slug"), required("name", "Name")}, creatorFlags()...),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				ci, err := creatorInput(a)
				if err != nil {
					return nil, err
				}
				return c.Studio.CreateCreator(ctx, ci)
			}},
		def{name: "update", short: "Edit a creator profile (only the flags given)", flags: append(append(argID("creator ID"), flag("slug", "URL slug"), flag("name", "Name")), creatorFlags()...),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				ci, err := creatorInput(a)
				if err != nil {
					return nil, err
				}
				return c.Studio.PatchCreator(ctx, a.S("id"), ci)
			}},
		def{name: "delete", short: "Delete a creator profile", flags: argID("creator ID"), destructive: true,
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return nil, c.Studio.DeleteCreator(ctx, a.S("id"))
			}},
	)
}

func userCommands() (*cobra.Command, error) {
	setDisabled := func(disabled bool) func(ctx context.Context, c *client.Client, a in) (any, error) {
		return func(ctx context.Context, c *client.Client, a in) (any, error) {
			return nil, c.Admin.PatchUser(ctx, a.S("id"), client.UserPatch{Disabled: &disabled})
		}
	}
	return group("users", "Customers and accounts (scopes admin:read, admin:write)",
		def{name: "list", short: "Search customers", flags: []*fieldsDef{flag("q", "Email, name, user ID or subject"), flag("role", "subscriber, creator or admin"), flag("plan", "Plan key"), integer("limit", 0, "At most this many (≤500)")},
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				rows, err := c.Admin.Users(ctx, client.UserQuery{Q: a.S("q"), Role: a.S("role"), Plan: a.S("plan"), Limit: a.I("limit")})
				out := []map[string]any{}
				for _, r := range rows {
					out = append(out, map[string]any{"id": r.User.ID, "email": r.User.Email, "name": r.User.Name, "role": r.User.Role, "disabled": r.User.Disabled,
						"plan": r.PlanKey, "source": r.Source, "access": r.Access.Reason, "until": r.Access.Until})
				}
				return out, err
			}},
		def{name: "show", short: "Show everything about a customer", flags: argID("user ID"),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) { return c.Admin.User(ctx, a.S("id")) }},
		def{name: "set-role", short: "Set a user's role", flags: []*fieldsDef{arg("id", "user ID"), arg("role", "subscriber, creator or admin")}, destructive: true,
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				role := a.S("role")
				return nil, c.Admin.PatchUser(ctx, a.S("id"), client.UserPatch{Role: &role})
			}},
		def{name: "disable", short: "Disable an account", flags: argID("user ID"), destructive: true, run: setDisabled(true)},
		def{name: "enable", short: "Re-enable an account", flags: argID("user ID"), run: setDisabled(false)},
		def{name: "revoke-sessions", short: "End a user's sessions and live views", flags: append(argID("user ID"), boolean("tokens", "Also revoke all API tokens")), destructive: true,
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return c.Admin.RevokeSessions(ctx, a.S("id"), a.B("tokens"))
			}},
		def{name: "grant", short: "Give complimentary access",
			flags: append(argID("user ID"), integer("days", 0, "Length in days (1–3660)"), required("reason", "Why; stored in the audit log"), flag("plan", "Plan key (default: the highest plan)")),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return c.Admin.GrantAccess(ctx, a.S("id"), client.GrantRequest{Days: a.I("days"), Reason: a.S("reason"), Plan: a.S("plan")})
			}},
		def{name: "revoke-grant", short: "Revoke one complimentary grant", flags: argID("grant ID"), destructive: true,
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return c.Admin.RevokeGrant(ctx, a.S("id"))
			}},
		def{name: "tokens", short: "List a user's API tokens", flags: argID("user ID"),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return c.Admin.UserTokens(ctx, a.S("id"))
			}},
		def{name: "revoke-token", short: "Revoke any user's API token", flags: argID("token ID"), destructive: true,
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return nil, c.Admin.RevokeToken(ctx, a.S("id"))
			}},
	)
}

func subscriptionCommands() (*cobra.Command, error) {
	cancel := func(v bool) func(ctx context.Context, c *client.Client, a in) (any, error) {
		return func(ctx context.Context, c *client.Client, a in) (any, error) {
			return nil, c.Admin.SetCancelAtPeriodEnd(ctx, a.S("id"), v)
		}
	}
	return group("subscriptions", "Stripe subscriptions (refunds and price changes stay in the Stripe dashboard)",
		def{name: "list", short: "List subscriptions", flags: []*fieldsDef{flag("status", "active, trialing, past_due, canceled, …"), integer("limit", 0, "At most this many")},
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return c.Admin.Subscriptions(ctx, a.S("status"), a.I("limit"))
			}},
		def{name: "cancel", short: "End a subscription at the end of its period", flags: argID("subscription ID"), destructive: true, run: cancel(true)},
		def{name: "resume", short: "Withdraw a scheduled cancellation", flags: argID("subscription ID"), run: cancel(false)},
	)
}

func instanceCommands() (*cobra.Command, error) {
	suspend := func(v bool) func(ctx context.Context, c *client.Client, a in) (any, error) {
		return func(ctx context.Context, c *client.Client, a in) (any, error) {
			return nil, c.Admin.SetInstanceSuspended(ctx, a.S("id"), v)
		}
	}
	return group("instances", "All instances (admin scopes)",
		def{name: "list", short: "List instances", flags: []*fieldsDef{flag("q", "Instance ID, owner, title, owner email or artifact")},
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return c.Admin.Instances(ctx, a.S("q"))
			}},
		def{name: "show", short: "Show an instance", flags: argID("instance ID"),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return c.Admin.Instance(ctx, a.S("id"))
			}},
		def{name: "suspend", short: "Suspend an instance", flags: argID("instance ID"), destructive: true, run: suspend(true)},
		def{name: "resume", short: "Resume an instance", flags: argID("instance ID"), run: suspend(false)},
		def{name: "delete", short: "Delete an instance and its data", flags: argID("instance ID"), destructive: true,
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return nil, c.Admin.DeleteInstance(ctx, a.S("id"))
			}},
	)
}

func planCommands() (*cobra.Command, error) {
	return group("plans", "Subscription plans in the settings document (admin scopes)",
		def{name: "list", short: "List plans", run: func(ctx context.Context, c *client.Client, _ in) (any, error) {
			s, err := c.Admin.Settings(ctx)
			return s.Active.Document.Plans, err
		}},
		def{name: "show", short: "Show a plan", flags: argID("plan key"),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				s, err := c.Admin.Settings(ctx)
				if err != nil {
					return nil, err
				}
				p, ok := s.Active.Document.Plan(a.S("id"))
				if !ok {
					return nil, errors.Errorf("no plan %s", a.S("id"))
				}
				return p, nil
			}},
		def{name: "set", short: "Change a plan (only the flags given); safe against concurrent edits",
			flags: append(argID("plan key"), flag("name", "Name"), flag("description", "Description"), flag("public", "true or false"),
				flag("instances", "Workspaces per subscriber"), flag("members", "Members per workspace"), flag("monthly-credits", "Monthly credits"),
				flag("daily-credits", "Daily credits"), flag("price-month", "Stripe price ID for monthly billing"), flag("price-month-label", "Label, e.g. €5 / month"),
				flag("price-year", "Stripe price ID for yearly billing"), flag("price-year-label", "Label, e.g. €50 / year")),
			destructive: true,
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return c.Admin.EditPlan(ctx, a.S("id"), func(p *settings.Plan) error { return applyPlanFlags(a, p) })
			}},
	)
}

func applyPlanFlags(a in, p *settings.Plan) error {
	if v := a.optS("name"); v != nil {
		p.Name = *v
	}
	if v := a.optS("description"); v != nil {
		p.Description = *v
	}
	pub, err := a.optB("public")
	if err != nil {
		return err
	}
	if pub != nil {
		p.Public = *pub
	}
	for name, dst := range map[string]*int{"instances": &p.Instances, "members": &p.MembersPerInstance, "monthly-credits": &p.MonthlyCredits, "daily-credits": &p.DailyCredits} {
		n, err := a.optI(name)
		if err != nil {
			return err
		}
		if n != nil {
			*dst = *n
		}
	}
	for _, interval := range []string{"month", "year"} {
		id := a.S("price-" + interval)
		if id == "" {
			continue
		}
		price := settings.PlanPrice{Interval: interval, PriceID: id, Label: a.S("price-" + interval + "-label")}
		replaced := false
		for i := range p.Prices {
			if p.Prices[i].Interval == interval {
				if price.Label == "" {
					price.Label = p.Prices[i].Label
				}
				p.Prices[i], replaced = price, true
			}
		}
		if !replaced {
			p.Prices = append(p.Prices, price)
		}
	}
	return nil
}

func settingsCommands() (*cobra.Command, error) {
	return group("settings", "The versioned settings document (admin scopes)",
		def{name: "show", short: "Show the active settings revision", run: func(ctx context.Context, c *client.Client, _ in) (any, error) {
			s, err := c.Admin.Settings(ctx)
			return s.Active, err
		}},
		def{name: "history", short: "Show recent settings revisions", run: func(ctx context.Context, c *client.Client, _ in) (any, error) {
			s, err := c.Admin.Settings(ctx)
			return s.History, err
		}},
		def{name: "apply", short: "Validate and activate a settings document from a file", destructive: true,
			flags: []*fieldsDef{required("file", "JSON settings document"), integer("base", -1, "Revision the file was edited from (default: the current one)")},
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				raw, err := os.ReadFile(a.S("file")) // #nosec G304 -- the operator names the file
				if err != nil {
					return nil, err
				}
				doc, err := settings.Parse(raw)
				if err != nil {
					return nil, err
				}
				base := int64(a.I("base"))
				if base < 0 {
					cur, err := c.Admin.Settings(ctx)
					if err != nil {
						return nil, err
					}
					base = cur.Active.Revision
				}
				return c.Admin.SaveSettings(ctx, doc, base)
			}},
	)
}

func billingCommands() (*cobra.Command, error) {
	return group("billing", "Stripe event inbox and reconciliation (admin scopes)",
		def{name: "status", short: "Show billing health, events and checkouts", flags: []*fieldsDef{flag("state", "pending, processed, ignored or error")},
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return c.Admin.Billing(ctx, a.S("state"))
			}},
		def{name: "reconcile", short: "Re-sync a customer from Stripe", flags: argID("user ID"),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return nil, c.Admin.Reconcile(ctx, a.S("id"))
			}},
		def{name: "retry-event", short: "Retry a failed Stripe event", flags: argID("event ID"),
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return nil, c.Admin.RetryEvent(ctx, a.S("id"))
			}},
	)
}

func usageCommands() (*cobra.Command, error) {
	return group("usage", "Model usage and credits (admin scopes)",
		def{name: "summary", short: "Usage summary and recent model calls", flags: []*fieldsDef{flag("user", "Only this user"), integer("limit", 0, "Calls to show")},
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return c.Admin.Usage(ctx, a.S("user"), a.I("limit"))
			}},
		def{name: "adjust", short: "Add or remove credits for the current period",
			flags: append(argID("user ID"), integer("credits", 0, "Credits to add (negative removes)"), required("reason", "Why; stored in the audit log")), destructive: true,
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return nil, c.Admin.AdjustCredits(ctx, a.S("id"), a.I("credits"), a.S("reason"))
			}},
	)
}

func auditCommands() (*cobra.Command, error) {
	return group("audit", "The audit log (admin scopes)",
		def{name: "list", short: "List audit entries, newest first",
			flags: []*fieldsDef{flag("subject", "Subject ID"), flag("actor", "Acting user ID"), flag("action", "Action, e.g. user.access_grant"), integer("before", 0, "Entries older than this entry ID"), integer("limit", 0, "At most this many (≤500)")},
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return c.Admin.Audit(ctx, client.AuditQuery{Subject: a.S("subject"), Actor: a.S("actor"), Action: a.S("action"), Before: int64(a.I("before")), Limit: a.I("limit")})
			}},
	)
}

func accountCommands() (*cobra.Command, error) {
	return group("account", "Your own account and instances (scope account)",
		def{name: "show", short: "Show your plan, subscriptions and allowance", run: func(ctx context.Context, c *client.Client, _ in) (any, error) { return c.Account.Get(ctx) }},
		def{name: "instances", short: "List your instances", run: func(ctx context.Context, c *client.Client, _ in) (any, error) { return c.Account.Instances(ctx) }},
		def{name: "create-instance", short: "Create a private instance of a release", flags: []*fieldsDef{required("release", "Release ID"), flag("title", "Title")},
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return c.Account.CreateInstance(ctx, a.S("release"), a.S("title"))
			}},
		def{name: "rename-instance", short: "Rename an instance", flags: []*fieldsDef{arg("id", "instance ID"), arg("title", "new title")},
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return nil, c.Account.RenameInstance(ctx, a.S("id"), a.S("title"))
			}},
		def{name: "delete-instance", short: "Delete your instance and its data", flags: argID("instance ID"), destructive: true,
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return nil, c.Account.DeleteInstance(ctx, a.S("id"))
			}},
		def{name: "add-member", short: "Share an instance", flags: []*fieldsDef{arg("id", "instance ID"), arg("email", "subscriber's email"), flag("role", "editor or viewer (default viewer)")},
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				role := a.S("role")
				if role == "" {
					role = "viewer"
				}
				return c.Account.AddMember(ctx, a.S("id"), a.S("email"), role)
			}},
		def{name: "remove-member", short: "Stop sharing an instance with someone", flags: []*fieldsDef{arg("id", "instance ID"), arg("user", "member's user ID")}, destructive: true,
			run: func(ctx context.Context, c *client.Client, a in) (any, error) {
				return nil, c.Account.RemoveMember(ctx, a.S("id"), a.S("user"))
			}},
	)
}
