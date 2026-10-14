#!/usr/bin/env bash
# Seed a local storefront database for visual checks: two creators, two
# published artifacts with covers and write-ups, and a journal post.
# Usage: 01-seed-dev-storefront.sh <database-url> <release-dir>
# The database must be migrated (francois catalog migrate). Local use only.
set -euo pipefail
DB=$1
REL=$2
root=$(git rev-parse --show-toplevel)
DB_NAME=${DB##*/}; DB_NAME=${DB_NAME%%\?*}
psql_() { docker exec -i francois-catalog-pg psql -q -U francois -d "$DB_NAME" -v ON_ERROR_STOP=1 "$@"; }

psql_ <<'SQL'
INSERT INTO users(id,issuer,subject,email,email_verified,display_name,role) VALUES
 ('usr_dev_admin','https://dev.local','admin','admin@dev.local',true,'Dev Admin','admin'),
 ('usr_dev_manuel','https://dev.local','manuel','manuel@dev.local',true,'Manuel','creator'),
 ('usr_dev_ramon','https://dev.local','ramon','ramon@dev.local',true,'Ramon','creator')
ON CONFLICT DO NOTHING;
SQL

publish() {
  (cd "$root" && go run ./cmd/francois catalog publish --database-url "$DB" --release-dir "$REL" \
    --artifact-id "$1" --title "$2" --version 1.0.0 --html "$3" --manifest "$4" --actor-user-id usr_dev_admin >/dev/null 2>&1) || echo "release $1 1.0.0 exists or failed; continuing"
}
publish polis "Polis IDE" examples/polis.html examples/polis.manifest.json
publish capabilities "Capabilities tour" examples/capabilities.html examples/capabilities.manifest.json

tmp=$(mktemp -d)
media() { # id file alt
  local sum key
  sum=$(sha256sum "$2" | cut -d' ' -f1)
  key="blobs/${sum:0:2}/$sum"
  mkdir -p "$REL/blobs/${sum:0:2}"
  cp "$2" "$REL/$key"
  local w h; w=$(identify -format '%w' "$2"); h=$(identify -format '%h' "$2")
  psql_ -c "INSERT INTO catalog_media(id,sha256,storage_key,media_type,size,width,height,alt,uploaded_by) VALUES ('$1','$sum','$key','image/png',$(stat -c %s "$2"),$w,$h,'$3','usr_dev_admin') ON CONFLICT DO NOTHING"
}
convert -size 1600x1000 gradient:'#e9e4d8-#b9b2a3' -gravity center -pointsize 140 -fill '#111' -annotate 0 'POLIS' "$tmp/polis.png"
convert -size 1600x1000 gradient:'#dfe7ea-#9fb3ba' -gravity center -pointsize 120 -fill '#111' -annotate 0 'CAPABILITIES' "$tmp/caps.png"
convert -size 1200x800 plasma:fractal -blur 0x6 "$tmp/shot.png"
convert -size 400x400 gradient:'#c9b8a6-#7a6a5a' "$tmp/manuel.png"
media med_dev_polis "$tmp/polis.png" "Polis cover"
media med_dev_caps "$tmp/caps.png" "Capabilities cover"
media med_dev_shot "$tmp/shot.png" "A session in Polis"
media med_dev_manuel "$tmp/manuel.png" "Manuel"
rm -rf "$tmp"

psql_ <<'SQL'
INSERT INTO creators(id,slug,name,bio,avatar_media_id,links,user_id,sort_order) VALUES
 ('cr_dev_manuel','manuel','Manuel','Builds tools for thinking with language models. **Polis** started as a way to run structured debates.','med_dev_manuel','[{"label":"scapegoat.dev","url":"https://scapegoat.dev"}]','usr_dev_manuel',0),
 ('cr_dev_ramon','ramon','Ramon','Designs interfaces that stay out of the way.',NULL,'[]','usr_dev_ramon',1)
ON CONFLICT DO NOTHING;
UPDATE catalog_artifacts SET description='A structured debate room where models argue and you decide.', tags='{agents,writing}', featured=true, sort_order=0,
  page='{"body":"## What it is\n\nPolis runs a **structured debate** between model personas. You set the question, pick the panel and watch positions converge.\n\n![A session in Polis](/media/med_dev_shot)\n\n## How to use it\n\n1. Write the question.\n2. Choose the panel.\n3. Read the synthesis.\n\n| Panel | Models |\n|---|---|\n| quick | GLM Flash |\n| deep | GLM 5.3 |\n","cover":"med_dev_polis","gallery":["med_dev_shot"]}'
  WHERE id='polis';
UPDATE catalog_artifacts SET description='Every runtime capability in one page: documents, rooms, assets and comments.', tags='{tools}', featured=true, sort_order=1,
  page='{"body":"A tour of `window.claude`.","cover":"med_dev_caps","gallery":[]}' WHERE id='capabilities';
INSERT INTO artifact_creators(artifact_id,creator_id,position) VALUES ('polis','cr_dev_manuel',0),('polis','cr_dev_ramon',1),('capabilities','cr_dev_ramon',0) ON CONFLICT DO NOTHING;
INSERT INTO posts(id,slug,title,summary,body,cover_media_id,author_id,tags,state,published_at,created_by) VALUES
 ('post_dev_1','why-single-file','Why single-file applications','One HTML file, one origin, no build step.',E'Shipping an application as **one file** means…\n\n## Origins\n\nEach instance gets its own origin.','med_dev_shot','cr_dev_manuel','{notes}','published',now(),'usr_dev_manuel')
ON CONFLICT DO NOTHING;
SQL
echo "seeded $DB_NAME"
