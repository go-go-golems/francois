package catalog

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

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
	"asset":   web.PortalURL,
	"join":    strings.Join,
	"ordinal": func(n int) string { return fmt.Sprintf("%03d", n) },
	"shortDate": func(t *time.Time) string {
		if t == nil {
			return ""
		}
		return t.UTC().Format("2 Jan")
	},
	"longDate": func(t *time.Time) string {
		if t == nil {
			return ""
		}
		return t.UTC().Format("2 January 2006")
	},
	"isVideo": content.IsVideo,
	"variant": content.VariantURL,
	"srcset":  srcset,
	// variantSet is a srcset for media whose width the page does not know
	// (video posters); widths past the original are served as the original.
	"variantSet": func(id string) string {
		parts := make([]string, len(content.VariantWidths))
		for i, w := range content.VariantWidths {
			parts[i] = fmt.Sprintf("%s %dw", content.VariantURL(id, w), w)
		}
		return strings.Join(parts, ", ")
	},
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
	JournalEnabled                                     bool
	JournalPreview                                     bool
	Supporter                                          bool
	Sensitive                                          bool
	LoginAvailable                                     bool
	LoginHref                                          string
	Draft                                              bool
	// TypeCredit shows the typeface credit when the licensed fonts are served.
	TypeCredit bool
	// OGType is the Open Graph type: "article" for journal posts, else
	// "website".
	OGType string
	// CardImage marks Image as a share card (content.CardWidth×CardHeight),
	// described by ImageAlt.
	CardImage bool
	ImageAlt  string
	// Contact is the configured public contact address, if any.
	Contact string
	// FontCSS and FontPreloads are versioned web-font URLs.
	FontCSS      string
	FontPreloads []string
	// Copy is the editable site text (settings storefront.copy).
	Copy siteCopy
	Body any
}

// siteCopy resolves editable site text for templates: {{.Copy.Text "key"}}
// for one-line text and {{.Copy.HTML "key"}} for Markdown.
type siteCopy struct {
	doc settings.Document
	md  *content.Markdown
}

func (c siteCopy) Text(key string) string { return c.doc.CopyText(key) }

func (c siteCopy) HTML(key string) template.HTML { return c.md.Render(c.doc.CopyText(key), nil) }

// Legal renders a legal page's Markdown with {site} and {contact} filled in.
func (c siteCopy) Legal(key, site, contact string) template.HTML {
	to := "the support address shown on the sign-in consent screen"
	if contact != "" {
		to = "[" + contact + "](mailto:" + contact + ")"
	}
	return c.md.Render(strings.NewReplacer("{site}", site, "{contact}", to).Replace(c.doc.CopyText(key)), nil)
}

// Date fills the {date} placeholder of a copy key.
func (c siteCopy) Date(key string, t *time.Time) string {
	if t == nil {
		return c.Text(key)
	}
	return strings.ReplaceAll(c.Text(key), "{date}", t.UTC().Format("2 January 2006"))
}

// card is an artifact as listed on the storefront.
type card struct {
	ID, Title, Description string
	Tags, Capabilities     []string
	Creators               []store.Creator
	Cover                  *store.Media
	Number                 int
	FirstPublished         *time.Time
	Version, ReleaseID     string
	Published              *time.Time
	Featured               bool
	EarlyAccessUntil       *time.Time
	DemoEnabled            bool
	LaunchMode             string
	// HTMLFile is the latest release's application file; FreeSample lets
	// anyone download it.
	HTMLFile   *store.ReleaseFile
	FreeSample bool
}

// EarlyAccess reports whether the card is still for paying customers only.
func (c card) EarlyAccess() bool {
	return c.EarlyAccessUntil != nil && c.EarlyAccessUntil.After(time.Now())
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
	d.JournalEnabled = rev.Document.Storefront.JournalEnabled
	d.Brand = s.brand(r, rev.Document)
	d.Copy = siteCopy{doc: rev.Document, md: s.md}
	d.TypeCredit = s.fonts.present
	d.Canonical = site.PortalOrigin() + canonicalPath(r.URL.Path)
	d.Contact = s.Config.ContactEmail
	d.FontCSS, d.FontPreloads = s.fonts.cssURL(), s.fonts.preloads()
	if d.OGType == "" {
		d.OGType = "website"
	}
	d.LoginAvailable = site.OIDC.Configured() || s.Config.EmailLogin != nil
	d.LoginHref = signinURL(r.URL.RequestURI())
	nav := []navItem{{Href: "/", Label: "Feed"}}
	// Demo guests browse like visitors: no account pages, and they may sign in.
	if p != nil && p.User.Role != "guest" {
		d.User = &p.User
		support, err := s.userSupport(r.Context(), p.User, rev.Document)
		if err != nil {
			s.pageFailed(w, r, err)
			return
		}
		d.Supporter = support.Active
		nav = append(nav, navItem{Href: "/account", Label: "Account"})
		if p.User.Role == "creator" || p.User.Role == "admin" {
			nav = append(nav, navItem{Href: "/studio", Label: "Studio"})
		}
		if p.User.Role == "admin" {
			nav = append(nav, navItem{Href: "/admin", Label: "Admin"})
		}
	}
	for i := range nav {
		nav[i].Current = r.URL.Path == nav[i].Href || (nav[i].Href != "/" && strings.HasPrefix(r.URL.Path, strings.TrimSuffix(nav[i].Href, "/")))
	}
	d.Nav = nav
	if d.Description == "" {
		d.Description = firstNonEmpty(rev.Document.Storefront.Tagline, rev.Document.CopyText("header.statement"))
	}
	if d.Image == "" {
		d.Image, d.CardImage, d.ImageAlt = s.shareURL(r, rev.Document), true, d.Brand
		if id := rev.Document.Storefront.ShareImage; id != "" {
			d.Image = s.site(r).PortalOrigin() + content.CardURL(id)
		}
	}
	var buf bytes.Buffer
	if err := s.pages[page].ExecuteTemplate(&buf, "layout", d); err != nil {
		log.Error().Err(err).Str("page", page).Msg("render page")
		s.unavailable(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// The header is personalised by the session cookie.
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Vary", "Cookie")
	if d.Draft || d.JournalPreview {
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	}
	if d.Sensitive {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	}
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusNotFound, "notfound", pageData{Title: "Not found"})
}

// canonicalPath folds the catalog's aliases of the homepage into "/".
func canonicalPath(p string) string {
	if p == "/apps" || p == "/apps/" {
		return "/"
	}
	return p
}

// pageFailed renders the unavailable page without the settings document or
// the session, either of which may be what failed.
func (s *Server) pageFailed(w http.ResponseWriter, r *http.Request, err error) {
	log.Error().Err(err).Str("path", r.URL.Path).Msg("storefront page")
	s.unavailable(w, r)
}

func (s *Server) unavailable(w http.ResponseWriter, r *http.Request) {
	d := pageData{Title: "Unavailable", Brand: s.site(r).PublicURL.Hostname(), OGType: "website", Contact: s.Config.ContactEmail,
		FontCSS: s.fonts.cssURL(), FontPreloads: s.fonts.preloads(), Canonical: s.site(r).PortalOrigin() + canonicalPath(r.URL.Path),
		Copy: siteCopy{doc: settings.Defaults(), md: s.md}}
	var buf bytes.Buffer
	if err := s.pages["unavailable"].ExecuteTemplate(&buf, "layout", d); err != nil {
		http.Error(w, "Page unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Retry-After", "30")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write(buf.Bytes())
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
	first := map[string]*time.Time{}
	for _, rel := range rels {
		if rel.PublishedAt == nil {
			continue
		}
		if first[rel.ArtifactID] == nil || rel.PublishedAt.Before(*first[rel.ArtifactID]) {
			first[rel.ArtifactID] = rel.PublishedAt
		}
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
		c.FirstPublished = first[a.ID]
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
	sort.Slice(out, func(i, j int) bool {
		if out[i].FirstPublished.Equal(*out[j].FirstPublished) {
			return out[i].ID < out[j].ID
		}
		return out[i].FirstPublished.Before(*out[j].FirstPublished)
	})
	for i := range out {
		out[i].Number = i + 1
	}
	return out, nil
}

func cardOf(a store.Artifact, rel store.Release, creators []store.Creator) card {
	c := card{ID: a.ID, Title: a.Title, Description: a.Description, Tags: a.Tags, Capabilities: releaseCaps(rel), Creators: creators,
		Version: rel.Version, ReleaseID: rel.ID, Published: rel.PublishedAt, Featured: a.Featured, EarlyAccessUntil: a.EarlyAccessUntil, DemoEnabled: a.DemoEnabled,
		LaunchMode: a.LaunchMode, FreeSample: rel.FreeSample}
	for i := range rel.Files {
		if rel.Files[i].Role == "html" {
			c.HTMLFile = &rel.Files[i]
			break
		}
	}
	return c
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

// feedPage serves the experiment archive, or published writing on /journal.
func (s *Server) feedPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.URL.Path == "/journal" {
		rev, err := s.Store.ActiveSettings(ctx)
		if err != nil {
			s.pageFailed(w, r, err)
			return
		}
		if !rev.Document.Storefront.JournalEnabled {
			s.notFound(w, r)
			return
		}
		posts, err := s.Store.Posts(ctx, "published", "", 0)
		if err != nil {
			s.pageFailed(w, r, err)
			return
		}
		s.render(w, r, http.StatusOK, "home", pageData{Title: "Journal", Body: map[string]any{"Journal": true, "Posts": posts}})
		return
	}
	cards, err := s.cards(ctx, listed)
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	rev, err := s.Store.ActiveSettings(ctx)
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	var who visitor
	if p := principal(ctx); p != nil {
		if e, _, err := s.entitlement(ctx, p.User); err == nil {
			who = visitor{member: e.Source != SourceGuest && e.Source != SourceDisabled, paying: e.Paid()}
		}
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	tag := r.URL.Query().Get("tag")
	order := r.URL.Query().Get("sort")
	if !archiveOrders[order] {
		order = "newest"
	}
	link := func(t string) string {
		v := url.Values{}
		if q != "" {
			v.Set("q", q)
		}
		if t != "" {
			v.Set("tag", t)
		}
		v.Set("sort", order)
		return "/?" + v.Encode()
	}
	type category struct {
		Name, Href string
		Count      int
		Current    bool
	}
	tags := []category{}
	index := map[string]int{}
	matches, hidden := []card{}, map[string]bool{}
	for _, c := range cards {
		for _, t := range c.Tags {
			if i, ok := index[t]; ok {
				tags[i].Count++
				continue
			}
			index[t] = len(tags)
			tags = append(tags, category{t, link(t), 1, t == tag})
		}
		haystack := strings.ToLower(c.Title + " " + c.Description + " " + strings.Join(c.Tags, " "))
		if tag == "" || contains(c.Tags, tag) {
			matches = append(matches, c)
			hidden[c.ID] = q != "" && !strings.Contains(haystack, strings.ToLower(q))
		}
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Name < tags[j].Name })
	rows, shown := make([]archiveRow, len(matches)), 0
	for i, c := range matches {
		rows[i] = archiveRowOf(c, rev.Document, who)
		if rows[i].Hidden = hidden[c.ID]; !rows[i].Hidden {
			shown++
		}
	}
	groups := groupArchive(rows, order)
	s.render(w, r, http.StatusOK, "home", pageData{Body: map[string]any{"Groups": groups, "Count": shown, "Total": len(cards), "Tags": tags, "Tag": tag,
		"Query": q, "Sort": order, "Orders": archiveOrderList, "AllHref": link(""), "Host": s.site(r).PublicURL.Host, "LoginError": r.URL.Query().Get("login_error")}})
}

// archiveOrderList is the landing page sort menu.
var archiveOrderList = []struct{ Value, Label string }{
	{"newest", "Newest first"}, {"oldest", "Oldest first"}, {"az", "A–Z"}, {"small", "Smallest first"}, {"large", "Largest first"},
}

var archiveOrders = map[string]bool{"newest": true, "oldest": true, "az": true, "small": true, "large": true}

// archiveRow is one experiment in the landing page list.
type archiveRow struct {
	card
	Size int64
	// Demo means a visitor without a plan may start a demo.
	Demo bool
	// Download is the file link, or a sign-in link for visitors who must
	// sign in first; empty during early access for visitors who do not pay.
	Download, DownloadName string
	// EarlyNote invites visitors who do not pay to subscribe.
	EarlyNote bool
	// Hidden rows do not match the search; they are rendered hidden so the
	// live search can show them again when the query broadens.
	Hidden bool
}

// visitor is what the landing page needs to know about the viewer: member
// means a signed-in account that may run artifacts (not a demo guest).
type visitor struct{ member, paying bool }

// archiveRowOf applies the download rule of mayDownload to a row: the link
// is hidden only while an artifact is in early access and the visitor does
// not pay. Visitors who must sign in first get a sign-in link instead of a
// download that would answer 401.
func archiveRowOf(c card, doc settings.Document, who visitor) archiveRow {
	row := archiveRow{card: c, Demo: doc.Demo.Enabled && c.DemoEnabled}
	if c.HTMLFile == nil {
		return row
	}
	row.Size, row.DownloadName = c.HTMLFile.Size, c.HTMLFile.Name
	switch {
	case c.EarlyAccess() && !who.paying:
		row.EarlyNote = true
	case c.FreeSample || who.member:
		row.Download = "/api/releases/" + c.ReleaseID + "/files/" + c.HTMLFile.ID + "/download"
	default:
		row.Download, row.DownloadName = signinURL("/apps/"+c.ID), ""
	}
	return row
}

// archiveGroup is a labelled run of rows: a month for date orders, an
// initial for A–Z, one group for size orders.
type archiveGroup struct {
	Label string
	Rows  []archiveRow
}

// Shown counts the group's rows that match the search.
func (g archiveGroup) Shown() int {
	n := 0
	for _, r := range g.Rows {
		if !r.Hidden {
			n++
		}
	}
	return n
}

func groupArchive(rows []archiveRow, order string) []archiveGroup {
	by := func(less func(a, b archiveRow) bool) {
		sort.SliceStable(rows, func(i, j int) bool { return less(rows[i], rows[j]) })
	}
	published := func(a, b archiveRow) bool {
		if a.FirstPublished.Equal(*b.FirstPublished) {
			return a.ID < b.ID
		}
		return a.FirstPublished.Before(*b.FirstPublished)
	}
	label := func(r archiveRow) string { return r.FirstPublished.UTC().Format("January 2006") }
	switch order {
	case "oldest":
		by(published)
	case "newest":
		by(func(a, b archiveRow) bool { return published(b, a) })
	case "az":
		by(func(a, b archiveRow) bool { return strings.ToLower(a.Title) < strings.ToLower(b.Title) })
		label = func(r archiveRow) string {
			first, _ := utf8.DecodeRuneInString(strings.ToUpper(r.Title))
			if !unicode.IsLetter(first) {
				return "#"
			}
			return string(first)
		}
	case "small", "large":
		by(func(a, b archiveRow) bool {
			if a.Size == b.Size {
				return a.Title < b.Title
			}
			return (a.Size < b.Size) == (order == "small")
		})
		label = func(archiveRow) string { return "By size" }
	}
	groups := []archiveGroup{}
	for _, r := range rows {
		if l := label(r); len(groups) == 0 || groups[len(groups)-1].Label != l {
			groups = append(groups, archiveGroup{Label: l})
		}
		groups[len(groups)-1].Rows = append(groups[len(groups)-1].Rows, r)
	}
	return groups
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

type download struct {
	Href, Label, Name string
	Size              int64
	Signin            bool // Href is the sign-in page, not the file
}

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
		for _, rel := range rels[1:] {
			if rel.PublishedAt.After(*latest.PublishedAt) {
				latest = rel
			}
		}
		// The public page offers the current release; history stays in Studio.
		rels = []store.Release{latest}
	}
	c := cardOf(a, latest, credits[a.ID])
	c.Number, c.FirstPublished, err = s.Store.ArchivePosition(ctx, a.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.pageFailed(w, r, err)
		return
	}
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
	mayDownload, paying, member := false, false, false
	if p != nil {
		if e, doc, err := s.entitlement(ctx, p.User); err == nil {
			mayDownload, paying = e.CanRun(a, doc), e.Paid()
			member = e.Source != SourceGuest && e.Source != SourceDisabled
		}
	}
	// The HTML download follows the landing page rule (archiveRowOf): none
	// while in early access for visitors who do not pay, a sign-in link for
	// visitors without an account, the file otherwise.
	hideDownload := a.EarlyAccess(time.Now()) && !paying
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
		for _, f := range rel.Files {
			switch {
			case f.Role != "html" || hideDownload:
			case !l.Locked:
				l.Downloads = append(l.Downloads, download{Href: "/api/releases/" + rel.ID + "/files/" + f.ID + "/download", Label: "Download ↓", Name: f.Name, Size: f.Size})
			case !member:
				l.Downloads = append(l.Downloads, download{Href: signinURL("/apps/" + a.ID), Label: "Download ↓", Name: f.Name, Size: f.Size, Signin: true})
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
	d := pageData{Title: a.Title, Description: firstNonEmpty(a.Description, content.Excerpt(page.Body, 200)), Crumb: "apps/" + a.ID, Draft: draft,
		Body: map[string]any{"Card": c, "Sections": s.md.Sections(page.Body, s.markdownEmbeds(ctx, page.Body)), "Gallery": gallery, "Releases": lines,
			"Demo": doc.Demo.Enabled && a.DemoEnabled && len(rels) > 0, "EarlyAccessUntil": a.EarlyAccessUntil, "EarlyAccess": a.EarlyAccess(time.Now()),
			"EarlyBlocked": a.EarlyAccess(time.Now()) && !paying}}
	if c.Cover != nil && !content.IsVideo(c.Cover.MediaType) {
		d.Image, d.CardImage, d.ImageAlt = s.site(r).PortalOrigin()+content.CardURL(c.Cover.ID), true, c.Cover.Alt
	} else if c.Cover != nil && c.Cover.Poster != "" {
		d.Image, d.CardImage, d.ImageAlt = s.site(r).PortalOrigin()+content.CardURL(c.Cover.Poster), true, a.Title
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
	rev, err := s.Store.ActiveSettings(ctx)
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	var posts []store.Post
	if rev.Document.Storefront.JournalEnabled {
		posts, err = s.Store.Posts(ctx, "published", c.ID, 0)
		if err != nil {
			s.pageFailed(w, r, err)
			return
		}
	}
	d := pageData{Title: c.Name, Crumb: "creators/" + c.Slug, Description: content.Excerpt(c.Bio, 200),
		Body: map[string]any{"Creator": c, "Bio": s.markdown(ctx, c.Bio), "Cards": cards, "Posts": posts}}
	if c.Avatar != "" {
		d.Image = s.site(r).PortalOrigin() + content.MediaPrefix + c.Avatar
	}
	s.render(w, r, http.StatusOK, "creator", d)
}

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
	rev, err := s.Store.ActiveSettings(ctx)
	if err != nil {
		s.pageFailed(w, r, err)
		return
	}
	pr := principal(ctx)
	hidden := !rev.Document.Storefront.JournalEnabled
	if hidden && (pr == nil || !s.canEditPost(ctx, pr.User, post)) {
		s.notFound(w, r)
		return
	}
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
	d := pageData{Title: post.Title, Crumb: "journal/" + post.Slug, Description: firstNonEmpty(post.Summary, content.Excerpt(post.Body, 200)), Draft: draft, JournalPreview: hidden, OGType: "article", Body: body}
	if post.Cover != "" {
		if m, err := s.Store.Media(ctx, post.Cover); err == nil {
			body["Cover"] = m
			if !content.IsVideo(m.MediaType) {
				d.Image, d.CardImage, d.ImageAlt = s.site(r).PortalOrigin()+content.CardURL(m.ID), true, m.Alt
			}
		}
	}
	s.render(w, r, http.StatusOK, "post", d)
}

type rssGUID struct {
	Value     string `xml:",chardata"`
	Permalink bool   `xml:"isPermaLink,attr"`
}

type rssItem struct {
	Title       string  `xml:"title"`
	Link        string  `xml:"link"`
	GUID        rssGUID `xml:"guid"`
	PubDate     string  `xml:"pubDate"`
	Description string  `xml:"description"`
}

type rssDoc struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	AtomNS  string   `xml:"xmlns:atom,attr"`
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Self        atomLink  `xml:"atom:link"`
		Description string    `xml:"description"`
		Items       []rssItem `xml:"item"`
	} `xml:"channel"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

func (s *Server) journalFeed(w http.ResponseWriter, r *http.Request) {
	rev, err := s.Store.ActiveSettings(r.Context())
	if err != nil {
		http.Error(w, "Feed unavailable", http.StatusServiceUnavailable)
		return
	}
	if !rev.Document.Storefront.JournalEnabled {
		s.notFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	posts, err := s.Store.Posts(r.Context(), "published", "", 50)
	if err != nil {
		http.Error(w, "Feed unavailable", http.StatusServiceUnavailable)
		return
	}
	origin := s.site(r).PortalOrigin()
	doc := rssDoc{Version: "2.0"}
	brand := s.brand(r, rev.Document)
	doc.Channel.Title = brand + " journal"
	doc.Channel.Link = origin + "/journal"
	doc.Channel.Description = "Notes on how the applications are made."
	for _, p := range posts {
		link := origin + "/journal/" + p.Slug
		item := rssItem{Title: p.Title, Link: link, GUID: rssGUID{Value: link, Permalink: true}, Description: firstNonEmpty(p.Summary, content.Excerpt(p.Body, 300))}
		if p.PublishedAt != nil {
			item.PubDate = p.PublishedAt.UTC().Format(time.RFC1123Z)
		}
		doc.Channel.Items = append(doc.Channel.Items, item)
	}
	writeRSS(w, doc, origin+r.URL.Path)
}

// media serves uploaded storefront media. A media ID always names the same
// bytes, so responses are cacheable indefinitely; ranges let video seek.
// markdown renders author Markdown, embedding referenced portal videos as
// players. If the media lookup fails, references render as images.
func (s *Server) markdown(ctx context.Context, source string) template.HTML {
	return s.md.Render(source, s.markdownEmbeds(ctx, source))
}

func (s *Server) markdownEmbeds(ctx context.Context, source string) map[string]content.Embed {
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
	return embeds
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
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	width, _ := strconv.Atoi(r.URL.Query().Get("w"))
	card := r.URL.Query().Get("card") == "1"
	if (card || content.VariantWidth(width)) && content.Resizable(m.MediaType, m.Width, m.Height) {
		name, resize := variantName(m, width), func(f io.ReadSeeker) ([]byte, error) {
			data, _, err := content.Resize(f, width)
			return data, err
		}
		if card {
			name, resize = m.SHA256+"-card-v"+strconv.Itoa(content.VariantVersion), content.Card
		}
		if data, err := s.variant(name, f, resize); err == nil {
			w.Header().Set("Content-Type", http.DetectContentType(data))
			w.Header().Set("ETag", `"`+name+`"`)
			http.ServeContent(w, r, "", m.CreatedAt, bytes.NewReader(data))
			return
		} else if !errors.Is(err, content.ErrNoVariant) {
			log.Warn().Err(err).Str("media", m.ID).Str("variant", name).Msg("media variant failed; serving original")
		}
		if _, err = f.Seek(0, io.SeekStart); err != nil {
			http.Error(w, "Media unavailable", http.StatusServiceUnavailable)
			return
		}
	}
	w.Header().Set("Content-Type", m.MediaType)
	w.Header().Set("ETag", `"`+m.SHA256+`"`)
	// ServeContent answers Range requests by seeking, so videos stream.
	http.ServeContent(w, r, "", m.CreatedAt, f)
}

// srcset lists the resized widths of a still image smaller than the
// original, then the original itself. It is empty when the image is not
// resized, so the template keeps a plain src.
func srcset(m store.Media) string {
	if !content.Resizable(m.MediaType, m.Width, m.Height) {
		return ""
	}
	var parts []string
	for _, w := range content.VariantWidths {
		if w < m.Width {
			parts = append(parts, fmt.Sprintf("%s %dw", content.VariantURL(m.ID, w), w))
		}
	}
	return strings.Join(append(parts, fmt.Sprintf("%s%s %dw", content.MediaPrefix, m.ID, m.Width)), ", ")
}

// variantName names a resized copy in the variant cache and its ETag; the
// version keeps copies from an older encoder from being served again.
func variantName(m store.Media, width int) string {
	return m.SHA256 + "-w" + strconv.Itoa(width) + "-v" + strconv.Itoa(content.VariantVersion)
}

// variant returns the cached resized copy of an image, creating it on first
// request. Concurrent requests for the same variant share one resize, and
// resizes run at most two at a time, so a burst of thumbnails cannot exhaust
// memory.
func (s *Server) variant(name string, f io.ReadSeeker, resize func(io.ReadSeeker) ([]byte, error)) ([]byte, error) {
	if data, err := s.Publisher.Blobs.Variant(name); err == nil {
		return data, nil
	}
	v, err, _ := s.resizes.Do(name, func() (any, error) {
		s.resizeSlots <- struct{}{}
		defer func() { <-s.resizeSlots }()
		data, err := resize(f)
		if err != nil {
			return nil, err
		}
		if err := s.Publisher.Blobs.PutVariant(name, data); err != nil {
			log.Warn().Err(err).Str("variant", name).Msg("media variant not cached")
		}
		return data, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]byte), nil
}

// pricingPage presents contributions; render supplies confirmed supporter state.
func (s *Server) pricingPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "pricing", pageData{Title: "Support the lab", Crumb: "pricing", Description: "Support an independent lab for computation and intelligence."})
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
		prices := []map[string]string{}
		for _, pr := range p.Prices {
			prices = append(prices, map[string]string{"interval": pr.Interval, "label": pr.Label})
		}
		out = append(out, map[string]any{"key": p.Key, "name": p.Name,
			"prices": prices, "instances": p.Instances, "membersPerInstance": p.MembersPerInstance, "dailyTokens": p.DailyTokens})
	}
	httpx.OK(w, out)
}
