package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pkg/errors"
)

// Page is the public page document of an artifact. The published copy is
// what visitors see; the draft is the editor's unpublished copy.
type Page struct {
	Body    string   `json:"body"`
	Cover   string   `json:"cover,omitempty"`
	Gallery []string `json:"gallery"`
}

type Artifact struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Visibility  string   `json:"visibility"`
	Page        Page     `json:"page"`
	PageDraft   *Page    `json:"pageDraft,omitempty"`
	Featured    bool     `json:"featured"`
	SortOrder   int      `json:"sortOrder"`
	// RequiredPlan is the key of the lowest plan that may run the artifact;
	// empty means the lowest paid plan.
	RequiredPlan string    `json:"requiredPlan"`
	DemoEnabled  bool      `json:"demoEnabled"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type ReleaseFile struct {
	ID         string `json:"id"`
	ReleaseID  string `json:"-"`
	Role       string `json:"role"`
	Name       string `json:"name"`
	MediaType  string `json:"mediaType"`
	Size       int64  `json:"size"`
	SHA256     string `json:"sha256"`
	StorageKey string `json:"-"`
}

type Release struct {
	ID          string                     `json:"id"`
	ArtifactID  string                     `json:"artifactId"`
	Version     string                     `json:"version"`
	Changelog   string                     `json:"changelog"`
	Manifest    map[string]json.RawMessage `json:"manifest"`
	State       string                     `json:"state"`
	FreeSample  bool                       `json:"freeSample"`
	CreatedBy   string                     `json:"createdBy"`
	CreatedAt   time.Time                  `json:"createdAt"`
	PublishedAt *time.Time                 `json:"publishedAt,omitempty"`
	WithdrawnAt *time.Time                 `json:"withdrawnAt,omitempty"`
	Files       []ReleaseFile              `json:"files"`
	Attachments []Attachment               `json:"attachments"`
}

// ErrConflict signals a uniqueness or state-transition conflict.
var ErrConflict = errors.New("conflict")

func isUnique(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

// CreateArtifact inserts a new artifact credited to creators in one
// transaction, so whoever creates an artifact is credited before anyone else
// can see it; an existing ID fails with ErrConflict.
func (s *Store) CreateArtifact(ctx context.Context, a Artifact, creators ...string) (Artifact, error) {
	if a.Tags == nil {
		a.Tags = []string{}
	}
	if a.Visibility == "" {
		a.Visibility = "listed"
	}
	if a.Page.Gallery == nil {
		a.Page.Gallery = []string{}
	}
	page, _ := json.Marshal(a.Page)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return a, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	err = tx.QueryRow(ctx, `INSERT INTO catalog_artifacts(id,title,description,tags,visibility,page,featured,sort_order,required_plan,demo_enabled)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING created_at,updated_at`,
		a.ID, a.Title, a.Description, a.Tags, a.Visibility, page, a.Featured, a.SortOrder, a.RequiredPlan, a.DemoEnabled).Scan(&a.CreatedAt, &a.UpdatedAt)
	if isUnique(err) {
		return a, errors.Wrapf(ErrConflict, "artifact %s already exists", a.ID)
	}
	if err != nil {
		return a, err
	}
	for i, c := range creators {
		if _, err = tx.Exec(ctx, `INSERT INTO artifact_creators(artifact_id,creator_id,position) VALUES($1,$2,$3)`, a.ID, c, i); err != nil {
			return a, err
		}
	}
	return a, tx.Commit(ctx)
}

// UpdateArtifact writes the settings that apply immediately. It never
// touches the page document or releases.
func (s *Store) UpdateArtifact(ctx context.Context, a Artifact) (Artifact, error) {
	if a.Tags == nil {
		a.Tags = []string{}
	}
	err := s.Pool.QueryRow(ctx, `UPDATE catalog_artifacts SET title=$2,description=$3,tags=$4,visibility=$5,featured=$6,sort_order=$7,required_plan=$8,demo_enabled=$9,updated_at=now()
WHERE id=$1 RETURNING updated_at`, a.ID, a.Title, a.Description, a.Tags, a.Visibility, a.Featured, a.SortOrder, a.RequiredPlan, a.DemoEnabled).Scan(&a.UpdatedAt)
	return a, notFound(err)
}

// SavePageDraft replaces the unpublished page document.
func (s *Store) SavePageDraft(ctx context.Context, id string, p Page) error {
	if p.Gallery == nil {
		p.Gallery = []string{}
	}
	raw, _ := json.Marshal(p)
	tag, err := s.Pool.Exec(ctx, `UPDATE catalog_artifacts SET page_draft=$2 WHERE id=$1`, id, raw)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// PublishPageDraft makes the draft the published page and clears the draft.
func (s *Store) PublishPageDraft(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE catalog_artifacts SET page=page_draft,page_draft=NULL,updated_at=now() WHERE id=$1 AND page_draft IS NOT NULL`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return errors.Wrap(ErrConflict, "there is no page draft")
	}
	return err
}

// DiscardPageDraft removes the unpublished page document.
func (s *Store) DiscardPageDraft(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE catalog_artifacts SET page_draft=NULL WHERE id=$1`, id)
	return err
}

const artifactColumns = `id,title,description,tags,visibility,page,page_draft,featured,sort_order,required_plan,demo_enabled,created_at,updated_at`

func scanArtifact(r pgx.Row) (Artifact, error) {
	var a Artifact
	var page, draft []byte
	err := r.Scan(&a.ID, &a.Title, &a.Description, &a.Tags, &a.Visibility, &page, &draft, &a.Featured, &a.SortOrder, &a.RequiredPlan, &a.DemoEnabled, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return a, notFound(err)
	}
	if err = json.Unmarshal(page, &a.Page); err != nil {
		return a, err
	}
	if a.Page.Gallery == nil {
		a.Page.Gallery = []string{}
	}
	if draft != nil {
		a.PageDraft = &Page{}
		if err = json.Unmarshal(draft, a.PageDraft); err != nil {
			return a, err
		}
		if a.PageDraft.Gallery == nil {
			a.PageDraft.Gallery = []string{}
		}
	}
	return a, nil
}

func (s *Store) Artifact(ctx context.Context, id string) (Artifact, error) {
	return scanArtifact(s.Pool.QueryRow(ctx, `SELECT `+artifactColumns+` FROM catalog_artifacts WHERE id=$1`, id))
}

// Artifacts lists every artifact in storefront order: featured first, then
// by sort order, then most recently updated.
func (s *Store) Artifacts(ctx context.Context) ([]Artifact, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+artifactColumns+` FROM catalog_artifacts ORDER BY featured DESC, sort_order, updated_at DESC`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Artifact, error) { return scanArtifact(r) })
}

// CreateDraft records a draft release whose files were already written to
// immutable storage. A duplicate artifact/version fails with ErrConflict.
func (s *Store) CreateDraft(ctx context.Context, r Release) (Release, error) {
	manifest, err := json.Marshal(r.Manifest)
	if err != nil {
		return r, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return r, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	r.ID = NewID("rel_")
	r.State = "draft"
	err = tx.QueryRow(ctx, `INSERT INTO releases(id,artifact_id,version,changelog,manifest,free_sample,created_by) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING created_at`,
		r.ID, r.ArtifactID, r.Version, r.Changelog, manifest, r.FreeSample, r.CreatedBy).Scan(&r.CreatedAt)
	if isUnique(err) {
		return r, errors.Wrapf(ErrConflict, "version %s already exists for %s", r.Version, r.ArtifactID)
	}
	if err != nil {
		return r, err
	}
	for i := range r.Files {
		f := &r.Files[i]
		f.ID = NewID("fil_")
		f.ReleaseID = r.ID
		if _, err = tx.Exec(ctx, `INSERT INTO release_files(id,release_id,role,name,media_type,size,sha256,storage_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
			f.ID, r.ID, f.Role, f.Name, f.MediaType, f.Size, f.SHA256, f.StorageKey); err != nil {
			return r, err
		}
	}
	return r, tx.Commit(ctx)
}

const releaseColumns = `id,artifact_id,version,changelog,manifest,state,free_sample,created_by,created_at,published_at,withdrawn_at`

func scanRelease(row pgx.Row) (Release, error) {
	var r Release
	var manifest []byte
	if err := row.Scan(&r.ID, &r.ArtifactID, &r.Version, &r.Changelog, &manifest, &r.State, &r.FreeSample, &r.CreatedBy, &r.CreatedAt, &r.PublishedAt, &r.WithdrawnAt); err != nil {
		return r, notFound(err)
	}
	return r, json.Unmarshal(manifest, &r.Manifest)
}

func (s *Store) files(ctx context.Context, releases []Release) error {
	if len(releases) == 0 {
		return nil
	}
	ids := make([]string, len(releases))
	index := map[string]int{}
	for i, r := range releases {
		ids[i] = r.ID
		index[r.ID] = i
		releases[i].Files = []ReleaseFile{}
	}
	rows, err := s.Pool.Query(ctx, `SELECT id,release_id,role,name,media_type,size,sha256,storage_key FROM release_files WHERE release_id=ANY($1) ORDER BY role`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var f ReleaseFile
		if err = rows.Scan(&f.ID, &f.ReleaseID, &f.Role, &f.Name, &f.MediaType, &f.Size, &f.SHA256, &f.StorageKey); err != nil {
			return err
		}
		i := index[f.ReleaseID]
		releases[i].Files = append(releases[i].Files, f)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	rows.Close()
	return s.attachments(ctx, releases, index)
}

func (s *Store) Release(ctx context.Context, id string) (Release, error) {
	r, err := scanRelease(s.Pool.QueryRow(ctx, `SELECT `+releaseColumns+` FROM releases WHERE id=$1`, id))
	if err != nil {
		return r, err
	}
	list := []Release{r}
	err = s.files(ctx, list)
	return list[0], err
}

// Releases lists an artifact's releases; states filters by state when given.
func (s *Store) Releases(ctx context.Context, artifact string, states ...string) ([]Release, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+releaseColumns+` FROM releases WHERE ($1='' OR artifact_id=$1) AND (COALESCE(cardinality($2::text[]),0)=0 OR state=ANY($2)) ORDER BY created_at DESC`, artifact, states)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Release, error) { return scanRelease(r) })
	if err != nil {
		return nil, err
	}
	return list, s.files(ctx, list)
}

// PublishRelease moves a draft to published exactly once and audits it.
func (s *Store) PublishRelease(ctx context.Context, id, actor string) (Release, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Release{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `UPDATE releases SET state='published',published_at=now(),published_by=$2 WHERE id=$1 AND state='draft'`, id, actor)
	if err != nil {
		return Release{}, err
	}
	if tag.RowsAffected() != 1 {
		return Release{}, errors.Wrap(ErrConflict, "only a draft can be published")
	}
	if _, err = tx.Exec(ctx, `UPDATE catalog_artifacts SET updated_at=now() WHERE id=(SELECT artifact_id FROM releases WHERE id=$1)`, id); err != nil {
		return Release{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO admin_audit(actor_id,token_id,action,subject,summary) VALUES($1,$3,'release.publish',$2,'{}')`, actor, id, auditToken(ctx)); err != nil {
		return Release{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Release{}, err
	}
	return s.Release(ctx, id)
}

// WithdrawRelease removes a published release from the catalog. When
// suspendPinned is set, active instances pinned to it are suspended.
func (s *Store) WithdrawRelease(ctx context.Context, id, actor string, suspendPinned bool) (int64, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `UPDATE releases SET state='withdrawn',withdrawn_at=now() WHERE id=$1 AND state='published'`, id)
	if err != nil {
		return 0, err
	}
	if tag.RowsAffected() != 1 {
		return 0, errors.Wrap(ErrConflict, "only a published release can be withdrawn")
	}
	var suspended int64
	if suspendPinned {
		tag, err = tx.Exec(ctx, `UPDATE instances SET state='suspended',updated_at=now() WHERE release_id=$1 AND state='active'`, id)
		if err != nil {
			return 0, err
		}
		suspended = tag.RowsAffected()
		if _, err = tx.Exec(ctx, `UPDATE runtime_grants SET revoked_at=now() WHERE instance_id IN (SELECT id FROM instances WHERE release_id=$1) AND revoked_at IS NULL`, id); err != nil {
			return 0, err
		}
	}
	summary, _ := json.Marshal(map[string]any{"suspendPinned": suspendPinned, "suspendedInstances": suspended})
	if _, err = tx.Exec(ctx, `INSERT INTO admin_audit(actor_id,token_id,action,subject,summary) VALUES($1,$4,'release.withdraw',$2,$3)`, actor, id, summary, auditToken(ctx)); err != nil {
		return 0, err
	}
	return suspended, tx.Commit(ctx)
}

// DeleteDraft removes an unpublished draft that no instance references.
func (s *Store) DeleteDraft(ctx context.Context, id string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `DELETE FROM instances WHERE release_id=$1 AND kind='preview'`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM release_files WHERE release_id=$1 AND EXISTS(SELECT 1 FROM releases WHERE id=$1 AND state='draft')`, id); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM releases WHERE id=$1 AND state='draft'`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.Wrap(ErrConflict, "only an unreferenced draft can be deleted")
	}
	return tx.Commit(ctx)
}

// ReferencedStorageKeys lists every blob key referenced by any release.
func (s *Store) ReferencedStorageKeys(ctx context.Context) ([]ReleaseFile, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id,release_id,role,name,media_type,size,sha256,storage_key FROM release_files ORDER BY release_id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (ReleaseFile, error) {
		var f ReleaseFile
		err := r.Scan(&f.ID, &f.ReleaseID, &f.Role, &f.Name, &f.MediaType, &f.Size, &f.SHA256, &f.StorageKey)
		return f, err
	})
}
