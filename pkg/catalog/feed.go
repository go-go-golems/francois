package catalog

import (
	"encoding/xml"
	"net/http"
	"net/url"
	"time"
)

// releaseFeed publishes only listed artifacts with published releases. IDs
// remain stable across title/changelog edits and different portal domains.
func (s *Server) releaseFeed(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.Pool.Query(r.Context(), `SELECT r.id,a.id,a.title,r.version,r.changelog,r.published_at FROM releases r JOIN catalog_artifacts a ON a.id=r.artifact_id WHERE r.state='published' AND a.visibility='listed' ORDER BY r.published_at DESC,r.id LIMIT 50`)
	if err != nil {
		http.Error(w, "Feed unavailable", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	origin := s.site(r).PortalOrigin()
	doc := rssDoc{Version: "2.0"}
	doc.Channel.Title = s.site(r).PublicURL.Hostname() + " releases"
	if rev, err := s.Store.ActiveSettings(r.Context()); err == nil {
		doc.Channel.Title = s.brand(r, rev.Document) + " releases"
	}
	doc.Channel.Link = origin + "/apps"
	doc.Channel.Description = "New application releases."
	for rows.Next() {
		var id, artifact, title, version, notes string
		var published time.Time
		if err = rows.Scan(&id, &artifact, &title, &version, &notes, &published); err != nil {
			http.Error(w, "Feed unavailable", http.StatusServiceUnavailable)
			return
		}
		doc.Channel.Items = append(doc.Channel.Items, rssItem{Title: title + " " + version, Link: origin + "/apps/" + url.PathEscape(artifact), GUID: rssGUID{Value: "urn:francois:release:" + id}, PubDate: published.UTC().Format(time.RFC1123Z), Description: notes})
	}
	if rows.Err() != nil {
		http.Error(w, "Feed unavailable", http.StatusServiceUnavailable)
		return
	}
	writeRSS(w, doc)
}
func writeRSS(w http.ResponseWriter, doc rssDoc) {
	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		http.Error(w, "Feed unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(append([]byte(xml.Header), out...))
}

// combinedFeed is the public subscription for both releases and journal posts.
func (s *Server) combinedFeed(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.Pool.Query(r.Context(), `SELECT id,title,path,description,published FROM (
 SELECT 'release:'||r.id AS id,a.title||' '||r.version AS title,'/apps/'||a.id AS path,r.changelog AS description,r.published_at AS published FROM releases r JOIN catalog_artifacts a ON a.id=r.artifact_id WHERE r.state='published' AND a.visibility='listed'
 UNION ALL SELECT 'post:'||id,title,'/journal/'||slug,summary,published_at FROM posts WHERE state='published'
 ) items ORDER BY published DESC,id LIMIT 50`)
	if err != nil {
		http.Error(w, "Feed unavailable", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	origin := s.site(r).PortalOrigin()
	doc := rssDoc{Version: "2.0"}
	doc.Channel.Title = s.site(r).PublicURL.Hostname()
	if rev, err := s.Store.ActiveSettings(r.Context()); err == nil {
		doc.Channel.Title = s.brand(r, rev.Document)
	}
	doc.Channel.Link = origin + "/"
	doc.Channel.Description = "Applications and writing."
	for rows.Next() {
		var id, title, path, description string
		var published time.Time
		if err = rows.Scan(&id, &title, &path, &description, &published); err != nil {
			http.Error(w, "Feed unavailable", http.StatusServiceUnavailable)
			return
		}
		doc.Channel.Items = append(doc.Channel.Items, rssItem{Title: title, Link: origin + path, GUID: rssGUID{Value: "urn:francois:" + id}, PubDate: published.UTC().Format(time.RFC1123Z), Description: description})
	}
	if rows.Err() != nil {
		http.Error(w, "Feed unavailable", http.StatusServiceUnavailable)
		return
	}
	writeRSS(w, doc)
}
