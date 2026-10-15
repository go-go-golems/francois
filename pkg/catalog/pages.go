package catalog

import (
	"bytes"
	"context"
	"encoding/xml"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/content"
	"github.com/go-go-golems/francois/pkg/catalog/httpx"
	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/go-go-golems/francois/pkg/catalog/web"
	"github.com/pkg/errors"
)

// Public storefront pages are rendered on the server so they are complete
// without JavaScript (search engines, link previews). Actions that need a
// CSRF-protected request are added by /static/public.js.

var pageFuncs = template.FuncMap{
	"join":    strings.Join,
	"isVideo": content.IsVideo,
	// period ends a title with a full stop unless it already ends in
	// punctuation ("Polis IDE." but "What is Polis?").
	"period": func(s string) string {
		s = strings.TrimSpace(s)
		if s == "" || strings.ContainsAny(s[len(s)-1:], ".!?…:") {
			return s
		}
		return s + "."
	},
	"size": func(n int64) string {
		switch {
		case n >= 1<<30:
			return strconv.FormatFloat(float64(n)/(1<<30), 'f', 1, 64) + " GB"
		case n >= 1<<20:
			return strconv.FormatFloat(float64(n)/(1<<20), 'f', 1, 64) + " MB"
		case n >= 1<<10:
			return strconv.FormatInt(n>>10, 10) + " KB"
		}
		return strconv.FormatInt(n, 10) + " B"
	},
	"date": func(v any) string {
		switch t := v.(type) {
		case time.Time:
			return t.UTC().Format("2 Jan 2006")
		case *time.Time:
			if t != nil {
				return t.UTC().Format("2 Jan 2006")
			}
		}
		return ""
	},
	"monogram": func(title string) string {
		letters := ""
		for _, w := range strings.Fields(title) {
			letters += strings.ToUpper(string([]rune(w)[0]))
			if len([]rune(letters)) == 3 {
				break
			}
		}
		return letters
	},
}

// parsePages parses each page template together with the shared layout.
func parsePages() map[string]*template.Template {
	names, err := fs.Glob(web.Templates, "templates/*.tmpl")
	if err != nil {
		panic(err)
	}
	pages := map[string]*template.Template{}
	for _, n := range names {
		name := strings.TrimSuffix(strings.TrimPrefix(n, "templates/"), ".tmpl")
		if name == "layout" {
			continue
		}
		pages[name] = template.Must(template.New(name).Funcs(pageFuncs).ParseFS(web.Templates, "templates/layout.tmpl", n))
	}
	return pages
}

type navItem struct {
	Href, Label string
	Current     bool
}

type pageData struct {
	Brand, Title, Description, Image, Canonical, Crumb string
	Nav                                                []navItem
	User                                               *store.User
	LoginAvailable                                     bool
	LoginHref                                          string
	Draft                                              bool
	// TypeCredit shows the typeface credit when the licensed fonts are served.
	TypeCredit bool
	Body       any
}

// card is an artifact as listed on the storefront.
type card struct {
	ID, Title, Description string
	Tags, Capabilities     []string
	Creators               []store.Creator
	Cover                  *store.Media
	Version, ReleaseID     string
	Published              *time.Time
	Featured               bool
	RequiredPlan           string
	DemoEnabled            bool
}

// brand is the storefront name shown in the header and titles: the
// configured storefront title, else the site's hostname.
func (s *Server) brand(r *http.Request, doc settings.Document) string {
	if doc.Storefront.Title != "" {
		return doc.Storefront.Title
	}
	return s.site(r).PublicURL.Hostname()
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, status int, page string, d pageData) {
	p := principal(r.Context())
	site := s.site(r)
	rev, err := s.Store.ActiveSettings(r.Context())
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	d.Brand = s.brand(r, rev.Document)
	d.TypeCredit = s.fonts.present
	d.Canonical = site.PortalOrigin() + r.URL.Path
	d.LoginAvailable = site.OIDC.Configured()
	d.LoginHref = "/auth/login?return=" + url.QueryEscape(r.URL.RequestURI())
	nav := []navItem{{Href: "/apps", Label: "Catalog"}, {Href: "/journal", Label: "Journal"}}
	if len(rev.Document.Prices()) > 0 {
		nav = append(nav, navItem{Href: "/pricing", Label: "Pricing"})
	}
	// Demo guests browse like visitors: no account pages, and they may sign in.
	if p != nil && p.User.Role != "guest" {
		d.User = &p.User
		nav = append(nav, navItem{Href: "/instances/", Label: "Instances"}, navItem{Href: "/account", Label: "Account"})
		if p.User.Role == "creator" || p.User.Role == "admin" {
			nav = append(nav, navItem{Href: "/studio", Label: "Studio"})
		}
		if p.User.Role == "admin" {
			nav = append(nav, navItem{Href: "/admin", Label: "Admin"})
		}
	}
	for i := range nav {
		nav[i].Current = strings.HasPrefix(r.URL.Path, strings.TrimSuffix(nav[i].Href, "/"))
	}
	d.Nav = nav
	if d.Description == "" {
		d.Description = rev.Document.Storefront.Tagline
	}
	var buf bytes.Buffer
	if err := s.pages[page].ExecuteTemplate(&buf, "layout", d); err != nil {
		log.Error().Err(err).Str("page", page).Msg("render page")
		http.Error(w, "Page unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// The header is personalised by the session cookie.
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Vary", "Cookie")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusNotFound, "notfound", pageData{Title: "Not found"})
}

func (s *Server) pageFailed(w http.ResponseWriter, r *http.Request, err error) {
	log.Error().Err(err).Str("path", r.URL.Path).Msg("storefront page")
	http.Error(w, "Page unavailable", http.StatusServiceUnavailable)
}

// releaseCaps lists the capabilities the hosted runtime provides for a
// release, in a stable order.
func releaseCaps(r store.Release) []string {
	caps := []string{}
	for _, k := range []string{"sample", "db", "room", "user", "assets", "comments", "downloads"} {
		if _, ok := r.Manifest[k]; ok {
			caps = append(caps, k)
		}
	}
	return caps
}

// cards returns every artifact that has a published release, in storefront
// order, with credits and covers resolved. include filters artifacts.
func (s *Server) cards(ctx context.Context, include func(store.Artifact) bool) ([]card, error) {
	arts, err := s.Store.Artifacts(ctx)
	if err != nil {
		return nil, err
	}
	rels, err := s.Store.Releases(ctx, "", "published")
	if err != nil {
		return nil, err
	}
	latest := map[string]store.Release{}
	for _, rel := range rels {
		if cur, ok := latest[rel.ArtifactID]; !ok || rel.PublishedAt.After(*cur.PublishedAt) {
			latest[rel.ArtifactID] = rel
		}
	}
	credits, err := s.Store.ArtifactCreators(ctx, nil)
	if err != nil {
		return nil, err
	}
	out := []card{}
	covers := []string{}
	for _, a := range arts {
		rel, ok := latest[a.ID]
		if !ok || (include != nil && !include(a)) {
			continue
		}
		c := cardOf(a, rel, credits[a.ID])
		out = append(out, c)
		if a.Page.Cover != "" {
			covers = append(covers, a.Page.Cover)
		}
	}
	media, err := s.Store.MediaByIDs(ctx, covers)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if a := find(arts, out[i].ID); a != nil {
			if m, ok := media[a.Page.Cover]; ok {
				out[i].Cover = &m
			}
		}
	}
	return out, nil
}

func cardOf(a store.Artifact, rel store.Release, creators []store.Creator) card {
	return card{ID: a.ID, Title: a.Title, Description: a.Description, Tags: a.Tags, Capabilities: releaseCaps(rel), Creators: creators,
		Version: rel.Version, ReleaseID: rel.ID, Published: rel.PublishedAt, Featured: a.Featured, RequiredPlan: a.RequiredPlan, DemoEnabled: a.DemoEnabled}
}

func find(arts []store.Artifact, id string) *store.Artifact {
	for i := range arts {
		if arts[i].ID == id {
			return &arts[i]
		}
	}
	return nil
}

func listed(a store.Artifact) bool { return a.Visibility == "listed" }

func (s *Server) homePage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	all, err := s.cards(ctx, listed)
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	featured := []card{}
	for _, c := range all {
		if c.Featured {
			featured = append(featured, c)
		}
	}
	if len(featured) == 0 && len(all) > 0 {
		featured = all[:min(len(all), 6)]
	}
	creators, err := s.Store.Creators(ctx)
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	posts, err := s.Store.Posts(ctx, "published", "", 3)
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	rev, err := s.Store.ActiveSettings(ctx)
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	front := rev.Document.Storefront
	// The front page leads with one statement; the brand is already in the
	// header's crumb.
	statement := firstNonEmpty(front.Tagline, front.Title, "Single-file HTML applications, hosted for you: a private origin, a real database, shared rooms and model calls — no setup, no keys.")
	body := map[string]any{
		"Statement":  statement,
		"Intro":      s.markdown(r.Context(), front.Intro),
		"Featured":   featured,
		"Creators":   creators,
		"Posts":      posts,
		"LoginError": r.URL.Query().Get("login_error"),
	}
	s.render(w, r, http.StatusOK, "home", pageData{Body: body})
}

type tagCount struct {
	Name    string
	Count   int
	Current bool
}

func (s *Server) catalogPage(w http.ResponseWriter, r *http.Request) {
	all, err := s.cards(r.Context(), listed)
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	tag := r.URL.Query().Get("tag")
	counts := map[string]int{}
	list := []card{}
	for _, c := range all {
		for _, t := range c.Tags {
			counts[t]++
		}
		if tag == "" || contains(c.Tags, tag) {
			list = append(list, c)
		}
	}
	tags := []tagCount{}
	for t, n := range counts {
		tags = append(tags, tagCount{Name: t, Count: n, Current: t == tag})
	}
	sort.Slice(tags, func(i, j int) bool {
		if tags[i].Count != tags[j].Count {
			return tags[i].Count > tags[j].Count
		}
		return tags[i].Name < tags[j].Name
	})
	s.render(w, r, http.StatusOK, "catalog", pageData{Title: "Applications", Crumb: "apps", Description: "All published applications.",
		Body: map[string]any{"Cards": list, "Tags": tags, "Tag": tag}})
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// canEditArtifact is the studio's object rule: administrators edit every
// artifact, creators those credited to their linked profile.
func (s *Server) canEditArtifact(ctx context.Context, u store.User, artifact string) bool {
	switch {
	case u.Disabled:
		return false
	case u.Role == "admin":
		return true
	case u.Role == "creator":
		ok, err := s.Store.IsArtifactCreator(ctx, artifact, u.ID)
		return err == nil && ok
	}
	return false
}

type download struct{ Href, Label string }

type releaseLine struct {
	store.Release
	Downloads []download
	// Attached lists the release's attachments; Href is empty when the
	// visitor may not download them (Locked).
	Attached []attachmentLine
	Locked   bool
}

type attachmentLine struct {
	Title, Name, Href string
	Size              int64
}

func (s *Server) artifactPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p := principal(ctx)
	a, err := s.Store.Artifact(ctx, r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		s.notFound(w, r)
		return
	}
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	draft := r.URL.Query().Get("draft") == "1" && p != nil && s.canEditArtifact(ctx, p.User, a.ID)
	rels, err := s.Store.Releases(ctx, a.ID, "published")
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	if !draft && (a.Visibility != "listed" || len(rels) == 0) {
		s.notFound(w, r)
		return
	}
	page := a.Page
	if draft && a.PageDraft != nil {
		page = *a.PageDraft
	}
	credits, err := s.Store.ArtifactCreators(ctx, []string{a.ID})
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	var latest store.Release
	if len(rels) > 0 {
		latest = rels[0]
	}
	c := cardOf(a, latest, credits[a.ID])
	ids := append([]string{page.Cover}, page.Gallery...)
	media, err := s.Store.MediaByIDs(ctx, ids)
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	if m, ok := media[page.Cover]; ok {
		c.Cover = &m
	}
	gallery := []store.Media{}
	for _, id := range page.Gallery {
		if m, ok := media[id]; ok {
			gallery = append(gallery, m)
		}
	}
	mayDownload := false
	if p != nil {
		if e, doc, err := s.entitlement(ctx, p.User); err == nil && e.CanRun(a, doc) {
			mayDownload = true
		}
	}
	lines := []releaseLine{}
	for _, rel := range rels {
		l := releaseLine{Release: rel, Locked: !rel.FreeSample && !mayDownload}
		for _, a := range rel.Attachments {
			al := attachmentLine{Title: firstNonEmpty(a.Label, a.Name), Name: a.Name, Size: a.Size}
			if !l.Locked {
				al.Href = "/api/releases/" + rel.ID + "/attachments/" + a.ID + "/download"
			}
			l.Attached = append(l.Attached, al)
		}
		if rel.FreeSample || mayDownload {
			for _, f := range rel.Files {
				label := "Download HTML"
				if f.Role == "manifest" {
					label = "Manifest"
				}
				l.Downloads = append(l.Downloads, download{Href: "/api/releases/" + rel.ID + "/files/" + f.ID + "/download", Label: label})
			}
		}
		lines = append(lines, l)
	}
	rev, err := s.Store.ActiveSettings(ctx)
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	doc := rev.Document
	rank := doc.RequiredRank(a.RequiredPlan)
	planName := ""
	for _, pl := range doc.Plans {
		if pl.Rank == rank {
			planName = pl.Name
		}
	}
	d := pageData{Title: a.Title, Description: firstNonEmpty(a.Description, content.Excerpt(page.Body, 200)), Crumb: "apps/" + a.ID, Draft: draft,
		Body: map[string]any{"Card": c, "Body": s.markdown(ctx, page.Body), "Gallery": gallery, "Releases": lines,
			"Demo": doc.Demo.Enabled && a.DemoEnabled && len(rels) > 0, "Plan": planName, "Rank": rank}}
	if c.Cover != nil && !content.IsVideo(c.Cover.MediaType) {
		d.Image = s.site(r).PortalOrigin() + content.MediaPrefix + c.Cover.ID
	} else if c.Cover != nil && c.Cover.Poster != "" {
		d.Image = s.site(r).PortalOrigin() + content.MediaPrefix + c.Cover.Poster
	}
	s.render(w, r, http.StatusOK, "artifact", d)
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func (s *Server) creatorPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c, err := s.Store.CreatorBySlug(ctx, r.PathValue("slug"))
	if errors.Is(err, store.ErrNotFound) {
		s.notFound(w, r)
		return
	}
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	ids, err := s.Store.CreatorArtifacts(ctx, c.ID)
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	cards, err := s.cards(ctx, func(a store.Artifact) bool { return listed(a) && contains(ids, a.ID) })
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	posts, err := s.Store.Posts(ctx, "published", c.ID, 0)
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	d := pageData{Title: c.Name, Crumb: "creators/" + c.Slug, Description: content.Excerpt(c.Bio, 200),
		Body: map[string]any{"Creator": c, "Bio": s.markdown(ctx, c.Bio), "Cards": cards, "Posts": posts}}
	if c.Avatar != "" {
		d.Image = s.site(r).PortalOrigin() + content.MediaPrefix + c.Avatar
	}
	s.render(w, r, http.StatusOK, "creator", d)
}

func (s *Server) journalPage(w http.ResponseWriter, r *http.Request) {
	posts, err := s.Store.Posts(r.Context(), "published", "", 0)
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "journal", pageData{Title: "Journal", Crumb: "journal", Description: "Notes on how the applications are made.", Body: posts})
}

// canEditPost: administrators, the post's creator account, and the account
// linked to its author profile.
func (s *Server) canEditPost(ctx context.Context, u store.User, p store.Post) bool {
	if u.Disabled {
		return false
	}
	if u.Role == "admin" {
		return true
	}
	if u.Role != "creator" {
		return false
	}
	if p.CreatedBy == u.ID {
		return true
	}
	c, err := s.Store.CreatorForUser(ctx, u.ID)
	return err == nil && p.AuthorID != "" && c.ID == p.AuthorID
}

func (s *Server) postPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	post, err := s.Store.PostBySlug(ctx, r.PathValue("slug"))
	if errors.Is(err, store.ErrNotFound) {
		s.notFound(w, r)
		return
	}
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	pr := principal(ctx)
	draft := post.State != "published"
	if draft && (pr == nil || !s.canEditPost(ctx, pr.User, post)) {
		s.notFound(w, r)
		return
	}
	body := map[string]any{"Post": post, "Body": s.markdown(ctx, post.Body)}
	var author *store.Creator
	if post.AuthorID != "" {
		if c, err := s.Store.Creator(ctx, post.AuthorID); err == nil {
			author = &c
		}
	}
	body["Author"] = author
	d := pageData{Title: post.Title, Crumb: "journal/" + post.Slug, Description: firstNonEmpty(post.Summary, content.Excerpt(post.Body, 200)), Draft: draft, Body: body}
	if post.Cover != "" {
		if m, err := s.Store.Media(ctx, post.Cover); err == nil {
			body["Cover"] = m
			if !content.IsVideo(m.MediaType) {
				d.Image = s.site(r).PortalOrigin() + content.MediaPrefix + m.ID
			}
		}
	}
	s.render(w, r, http.StatusOK, "post", d)
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
}

type rssDoc struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Items       []rssItem `xml:"item"`
	} `xml:"channel"`
}

func (s *Server) journalFeed(w http.ResponseWriter, r *http.Request) {
	posts, err := s.Store.Posts(r.Context(), "published", "", 50)
	if err != nil {
		http.Error(w, "Feed unavailable", http.StatusServiceUnavailable)
		return
	}
	origin := s.site(r).PortalOrigin()
	doc := rssDoc{Version: "2.0"}
	brand := s.site(r).PublicURL.Hostname()
	if rev, err := s.Store.ActiveSettings(r.Context()); err == nil {
		brand = s.brand(r, rev.Document)
	}
	doc.Channel.Title = brand + " journal"
	doc.Channel.Link = origin + "/journal"
	doc.Channel.Description = "Notes on how the applications are made."
	for _, p := range posts {
		link := origin + "/journal/" + p.Slug
		item := rssItem{Title: p.Title, Link: link, GUID: link, Description: firstNonEmpty(p.Summary, content.Excerpt(p.Body, 300))}
		if p.PublishedAt != nil {
			item.PubDate = p.PublishedAt.UTC().Format(time.RFC1123Z)
		}
		doc.Channel.Items = append(doc.Channel.Items, item)
	}
	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		http.Error(w, "Feed unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(out)
}

// media serves uploaded storefront media. A media ID always names the same
// bytes, so responses are cacheable indefinitely; ranges let video seek.
// markdown renders author Markdown, embedding referenced portal videos as
// players. If the media lookup fails, references render as images.
func (s *Server) markdown(ctx context.Context, source string) template.HTML {
	embeds := map[string]content.Embed{}
	if ids := content.MediaRefs(source); len(ids) > 0 {
		media, err := s.Store.MediaByIDs(ctx, ids)
		if err != nil {
			log.Warn().Err(err).Msg("Markdown media lookup failed")
		}
		for id, m := range media {
			embeds[id] = content.Embed{Type: m.MediaType, Poster: m.Poster}
		}
	}
	return s.md.Render(source, embeds)
}

func (s *Server) media(w http.ResponseWriter, r *http.Request) {
	m, err := s.Store.Media(r.Context(), r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	f, err := s.Publisher.Blobs.Open(m.StorageKey)
	if err != nil {
		log.Error().Err(err).Str("media", m.ID).Msg("media unreadable")
		http.Error(w, "Media unavailable", http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = f.Close() }()
	w.Header().Set("Content-Type", m.MediaType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("ETag", `"`+m.SHA256+`"`)
	// ServeContent answers Range requests by seeking, so videos stream.
	http.ServeContent(w, r, "", m.CreatedAt, f)
}

type planView struct {
	settings.Plan
	Current bool
}

// pricingPage lists the public plans by rank, marking the caller's plan.
func (s *Server) pricingPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rev, err := s.Store.ActiveSettings(ctx)
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	current, paid, staff := "", false, false
	if p := principal(ctx); p != nil {
		if e, err := s.entitlementIn(ctx, p.User, rev.Document); err == nil {
			current, paid, staff = e.Plan.Key, e.Source == SourceSubscription || e.Source == SourceTrial, e.Source == SourceStaff
		}
	}
	if staff {
		current = ""
	}
	plans := []planView{}
	name := ""
	for _, p := range rev.Document.Plans {
		if p.Key == current {
			name = p.Name
		}
		if p.Public {
			plans = append(plans, planView{Plan: p, Current: p.Key == current})
		}
	}
	sort.Slice(plans, func(i, j int) bool { return plans[i].Rank < plans[j].Rank })
	s.render(w, r, http.StatusOK, "pricing", pageData{Title: "Pricing", Crumb: "pricing", Description: "Plans and prices.",
		Body: map[string]any{"Plans": plans, "Current": name, "Paid": paid, "Staff": staff}})
}

// publicPlans is GET /api/plans: the public plans, without price IDs.
func (s *Server) publicPlans(w http.ResponseWriter, r *http.Request) {
	rev, err := s.Store.ActiveSettings(r.Context())
	if err != nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	out := []map[string]any{}
	for _, p := range rev.Document.Plans {
		if !p.Public {
			continue
		}
		prices := []map[string]string{}
		for _, pr := range p.Prices {
			prices = append(prices, map[string]string{"interval": pr.Interval, "label": pr.Label})
		}
		out = append(out, map[string]any{"key": p.Key, "name": p.Name, "rank": p.Rank, "description": p.Description, "highlights": p.Highlights,
			"prices": prices, "instances": p.Instances, "membersPerInstance": p.MembersPerInstance, "monthlyCredits": p.MonthlyCredits})
	}
	httpx.OK(w, out)
}
