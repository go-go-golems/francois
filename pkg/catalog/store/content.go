package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
)

// Media is a public image or video uploaded for storefront pages. The bytes
// live in the release blob store under StorageKey.
type Media struct {
	ID         string    `json:"id"`
	SHA256     string    `json:"sha256"`
	StorageKey string    `json:"-"`
	MediaType  string    `json:"mediaType"`
	Size       int64     `json:"size"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	Alt        string    `json:"alt"`
	UploadedBy string    `json:"uploadedBy"`
	CreatedAt  time.Time `json:"createdAt"`
}

const mediaColumns = `id,sha256,storage_key,media_type,size,width,height,alt,uploaded_by,created_at`

func scanMedia(r pgx.Row) (Media, error) {
	var m Media
	err := r.Scan(&m.ID, &m.SHA256, &m.StorageKey, &m.MediaType, &m.Size, &m.Width, &m.Height, &m.Alt, &m.UploadedBy, &m.CreatedAt)
	return m, notFound(err)
}

func (s *Store) CreateMedia(ctx context.Context, m Media) (Media, error) {
	m.ID = NewID("med_")
	err := s.Pool.QueryRow(ctx, `INSERT INTO catalog_media(id,sha256,storage_key,media_type,size,width,height,alt,uploaded_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING created_at`,
		m.ID, m.SHA256, m.StorageKey, m.MediaType, m.Size, m.Width, m.Height, m.Alt, m.UploadedBy).Scan(&m.CreatedAt)
	return m, err
}

func (s *Store) Media(ctx context.Context, id string) (Media, error) {
	return scanMedia(s.Pool.QueryRow(ctx, `SELECT `+mediaColumns+` FROM catalog_media WHERE id=$1`, id))
}

// MediaByIDs returns the media that exist among ids, keyed by ID.
func (s *Store) MediaByIDs(ctx context.Context, ids []string) (map[string]Media, error) {
	out := map[string]Media{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.Pool.Query(ctx, `SELECT `+mediaColumns+` FROM catalog_media WHERE id=ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Media, error) { return scanMedia(r) })
	for _, m := range list {
		out[m.ID] = m
	}
	return out, err
}

// MediaList lists media, newest first; uploader filters when not empty.
func (s *Store) MediaList(ctx context.Context, uploader string) ([]Media, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+mediaColumns+` FROM catalog_media WHERE ($1='' OR uploaded_by=$1) ORDER BY created_at DESC`, uploader)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Media, error) { return scanMedia(r) })
}

func (s *Store) SetMediaAlt(ctx context.Context, id, alt string) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE catalog_media SET alt=$2 WHERE id=$1`, id, alt)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// MediaReferenced reports whether any page, draft, creator or post refers to
// the media, either as a field or by its /media/<id> URL in Markdown.
func (s *Store) MediaReferenced(ctx context.Context, id string) (bool, error) {
	url := "%/media/" + id + "%"
	var used bool
	err := s.Pool.QueryRow(ctx, `SELECT
  EXISTS(SELECT 1 FROM catalog_artifacts WHERE page->>'cover'=$1 OR page->'gallery' ? $1 OR page->>'body' LIKE $2
                                         OR page_draft->>'cover'=$1 OR page_draft->'gallery' ? $1 OR page_draft->>'body' LIKE $2)
  OR EXISTS(SELECT 1 FROM creators WHERE avatar_media_id=$1 OR bio LIKE $2)
  OR EXISTS(SELECT 1 FROM posts WHERE cover_media_id=$1 OR body LIKE $2)`, id, url).Scan(&used)
	return used, err
}

// DeleteMedia removes an unreferenced media row. The blob stays: identical
// bytes may back other media or releases.
func (s *Store) DeleteMedia(ctx context.Context, id string) error {
	used, err := s.MediaReferenced(ctx, id)
	if err != nil {
		return err
	}
	if used {
		return errors.Wrap(ErrConflict, "the media is still used")
	}
	tag, err := s.Pool.Exec(ctx, `DELETE FROM catalog_media WHERE id=$1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// Link is a labelled external link on a creator profile.
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

// creatorCols lists creator columns qualified by a table alias ("" for none).
func creatorCols(t string) string {
	return t + "id," + t + "slug," + t + "name," + t + "bio,COALESCE(" + t + "avatar_media_id,'')," + t + "links,COALESCE(" + t + "user_id,'')," + t + "sort_order," + t + "created_at," + t + "updated_at"
}

var creatorColumns = creatorCols("")

func scanCreator(r pgx.Row) (Creator, error) {
	var c Creator
	var links []byte
	if err := r.Scan(&c.ID, &c.Slug, &c.Name, &c.Bio, &c.Avatar, &links, &c.UserID, &c.SortOrder, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return c, notFound(err)
	}
	if err := json.Unmarshal(links, &c.Links); err != nil {
		return c, err
	}
	if c.Links == nil {
		c.Links = []Link{}
	}
	return c, nil
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (s *Store) CreateCreator(ctx context.Context, c Creator) (Creator, error) {
	c.ID = NewID("cr_")
	if c.Links == nil {
		c.Links = []Link{}
	}
	links, _ := json.Marshal(c.Links)
	err := s.Pool.QueryRow(ctx, `INSERT INTO creators(id,slug,name,bio,avatar_media_id,links,user_id,sort_order) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING created_at,updated_at`,
		c.ID, c.Slug, c.Name, c.Bio, nullable(c.Avatar), links, nullable(c.UserID), c.SortOrder).Scan(&c.CreatedAt, &c.UpdatedAt)
	if isUnique(err) {
		return c, errors.Wrap(ErrConflict, "the slug or user is already used by another creator")
	}
	return c, err
}

func (s *Store) UpdateCreator(ctx context.Context, c Creator) (Creator, error) {
	if c.Links == nil {
		c.Links = []Link{}
	}
	links, _ := json.Marshal(c.Links)
	err := s.Pool.QueryRow(ctx, `UPDATE creators SET slug=$2,name=$3,bio=$4,avatar_media_id=$5,links=$6,user_id=$7,sort_order=$8,updated_at=now() WHERE id=$1 RETURNING updated_at`,
		c.ID, c.Slug, c.Name, c.Bio, nullable(c.Avatar), links, nullable(c.UserID), c.SortOrder).Scan(&c.UpdatedAt)
	if isUnique(err) {
		return c, errors.Wrap(ErrConflict, "the slug or user is already used by another creator")
	}
	return c, notFound(err)
}

func (s *Store) DeleteCreator(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM creators WHERE id=$1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) Creator(ctx context.Context, id string) (Creator, error) {
	return scanCreator(s.Pool.QueryRow(ctx, `SELECT `+creatorColumns+` FROM creators WHERE id=$1`, id))
}

func (s *Store) CreatorBySlug(ctx context.Context, slug string) (Creator, error) {
	return scanCreator(s.Pool.QueryRow(ctx, `SELECT `+creatorColumns+` FROM creators WHERE slug=$1`, slug))
}

// CreatorForUser is the profile linked to an account, if any.
func (s *Store) CreatorForUser(ctx context.Context, user string) (Creator, error) {
	return scanCreator(s.Pool.QueryRow(ctx, `SELECT `+creatorColumns+` FROM creators WHERE user_id=$1`, user))
}

func (s *Store) Creators(ctx context.Context) ([]Creator, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+creatorColumns+` FROM creators ORDER BY sort_order, name`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Creator, error) { return scanCreator(r) })
}

// SetArtifactCreators replaces an artifact's credited creators, in order.
func (s *Store) SetArtifactCreators(ctx context.Context, artifact string, creators []string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `DELETE FROM artifact_creators WHERE artifact_id=$1`, artifact); err != nil {
		return err
	}
	for i, c := range creators {
		if _, err = tx.Exec(ctx, `INSERT INTO artifact_creators(artifact_id,creator_id,position) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, artifact, c, i); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ArtifactCreators maps artifact IDs to their credited creators in order.
// An empty ids list returns the credits of every artifact.
func (s *Store) ArtifactCreators(ctx context.Context, ids []string) (map[string][]Creator, error) {
	rows, err := s.Pool.Query(ctx, `SELECT ac.artifact_id,`+creatorCols("c.")+` FROM artifact_creators ac JOIN creators c ON c.id=ac.creator_id
WHERE COALESCE(cardinality($1::text[]),0)=0 OR ac.artifact_id=ANY($1) ORDER BY ac.artifact_id, ac.position`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]Creator{}
	for rows.Next() {
		var artifact string
		var c Creator
		var links []byte
		if err = rows.Scan(&artifact, &c.ID, &c.Slug, &c.Name, &c.Bio, &c.Avatar, &links, &c.UserID, &c.SortOrder, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(links, &c.Links); err != nil {
			return nil, err
		}
		out[artifact] = append(out[artifact], c)
	}
	return out, rows.Err()
}

// CreatorArtifacts lists the IDs of artifacts credited to a creator.
func (s *Store) CreatorArtifacts(ctx context.Context, creator string) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `SELECT artifact_id FROM artifact_creators WHERE creator_id=$1`, creator)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// IsArtifactCreator reports whether the user is linked to a creator credited
// on the artifact.
func (s *Store) IsArtifactCreator(ctx context.Context, artifact, user string) (bool, error) {
	var ok bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM artifact_creators ac JOIN creators c ON c.id=ac.creator_id WHERE ac.artifact_id=$1 AND c.user_id=$2)`, artifact, user).Scan(&ok)
	return ok, err
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

const postColumns = `id,slug,title,summary,body,COALESCE(cover_media_id,''),COALESCE(author_id,''),tags,state,published_at,created_by,created_at,updated_at`

func scanPost(r pgx.Row) (Post, error) {
	var p Post
	err := r.Scan(&p.ID, &p.Slug, &p.Title, &p.Summary, &p.Body, &p.Cover, &p.AuthorID, &p.Tags, &p.State, &p.PublishedAt, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	return p, notFound(err)
}

func (s *Store) CreatePost(ctx context.Context, p Post) (Post, error) {
	p.ID = NewID("post_")
	p.State = "draft"
	if p.Tags == nil {
		p.Tags = []string{}
	}
	err := s.Pool.QueryRow(ctx, `INSERT INTO posts(id,slug,title,summary,body,cover_media_id,author_id,tags,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING created_at,updated_at`,
		p.ID, p.Slug, p.Title, p.Summary, p.Body, nullable(p.Cover), nullable(p.AuthorID), p.Tags, p.CreatedBy).Scan(&p.CreatedAt, &p.UpdatedAt)
	if isUnique(err) {
		return p, errors.Wrapf(ErrConflict, "slug %s is already used", p.Slug)
	}
	return p, err
}

// UpdatePost writes the editable fields; state changes use SetPostState.
func (s *Store) UpdatePost(ctx context.Context, p Post) (Post, error) {
	if p.Tags == nil {
		p.Tags = []string{}
	}
	err := s.Pool.QueryRow(ctx, `UPDATE posts SET slug=$2,title=$3,summary=$4,body=$5,cover_media_id=$6,author_id=$7,tags=$8,updated_at=now() WHERE id=$1 RETURNING updated_at`,
		p.ID, p.Slug, p.Title, p.Summary, p.Body, nullable(p.Cover), nullable(p.AuthorID), p.Tags).Scan(&p.UpdatedAt)
	if isUnique(err) {
		return p, errors.Wrapf(ErrConflict, "slug %s is already used", p.Slug)
	}
	return p, notFound(err)
}

// SetPostState publishes or unpublishes a post. The first publication time
// is kept when a post is republished.
func (s *Store) SetPostState(ctx context.Context, id, state string) (Post, error) {
	return scanPost(s.Pool.QueryRow(ctx, `UPDATE posts SET state=$2,published_at=CASE WHEN $2='published' THEN COALESCE(published_at,now()) ELSE published_at END,updated_at=now()
WHERE id=$1 RETURNING `+postColumns, id, state))
}

func (s *Store) DeletePost(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM posts WHERE id=$1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) Post(ctx context.Context, id string) (Post, error) {
	return scanPost(s.Pool.QueryRow(ctx, `SELECT `+postColumns+` FROM posts WHERE id=$1`, id))
}

func (s *Store) PostBySlug(ctx context.Context, slug string) (Post, error) {
	return scanPost(s.Pool.QueryRow(ctx, `SELECT `+postColumns+` FROM posts WHERE slug=$1`, slug))
}

// Posts lists posts newest first. state and author filter when not empty;
// limit 0 means no limit.
func (s *Store) Posts(ctx context.Context, state, author string, limit int) ([]Post, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+postColumns+` FROM posts WHERE ($1='' OR state=$1) AND ($2='' OR author_id=$2)
ORDER BY COALESCE(published_at,created_at) DESC LIMIT NULLIF($3,0)`, state, author, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Post, error) { return scanPost(r) })
}
