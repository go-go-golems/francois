---
Title: Implementation diary
Ticket: FRANCOIS-016
Status: active
Topics:
    - frontend
    - backend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-17T17:15:03.46672879-04:00
WhatFor: ""
WhenToUse: ""
---
# Implementation diary

## Step 1: Default the public blog off and wire the admin control

The user wants an artifact-focused public site before remote deployment, while retaining a way to enable the blog. I added one boolean to the existing versioned settings document, rather than a new configuration service or migration. The setting covers every existing public post surface and retains private editorial previews.

The focused database tests and a real admin browser round trip passed. The local preview was left with the setting false. A full-suite run exposed an existing concurrent first-artifact upload authorization race; that issue is separated from this feature and will be addressed before final qualification.

### Prompt Context

**User prompt (verbatim):** "ok, before deploying remotely, let's make the blog hidden per deafult (admin setting), and keep just the artifact"
**Assistant interpretation:** Implement a default-off public blog setting, expose it in Admin and apply it consistently before any production deployment.
**Inferred user intent:** Keep public presentation minimal and artifact-focused while preserving the editorial system.
**Starting revision:** 0cb9c35.

### What I did

- Added storefront.journalEnabled with a natural false default, including previously saved JSON documents where the field is absent.
- Added the Show the blog publicly checkbox under Admin Settings Storefront and the matching TypeScript contract.
- Gated /journal and /journal/feed.xml, public individual post access, creator-page posts and mixed RSS blog items. Artifact pages and release feeds remain available.
- Retained editor-authorized individual previews with a clear hidden-blog banner, private/no-store and noindex. Existing draft previews also receive private caching.
- Made the public footer conditional and returned the setting from /api/session for the React footer. Settings saves invalidate the Session query so navigation updates immediately.
- Made visibility-sensitive feeds revalidate rather than cache stale blog visibility for five minutes; artifact-only feed caching remains unchanged.
- Updated older journal tests to explicitly enable the feature they exercise. Added default-off/old-document/admin-toggle/preview/role/feed regression coverage.
- Added a local-only browser script and screenshots. It creates a temporary post/session, saves both checkbox states through the real UI, verifies anonymous post/feed access and cleans up afterward.
- Updated production documentation and added a cross-ticket visibility requirement to the proposed recursive catalog API ticket.

### Why

A hidden footer link alone would leave published posts reachable by URL or RSS. Enforcing the same flag at each public read surface gives the requested behavior. Editorial availability and public blog visibility are separate concerns; hiding the blog must not delete or unpublish content.

### What worked

- Focused database suite: TestJournal, TestFinal, TestStorefrontPages and TestCombinedFeed passed.
- pnpm check, pnpm test (9 tests) and pnpm build using Node v24.18.0/Corepack passed.
- go build ./... and make lint logcopter-check passed.
- Chromium confirmed the default-off checkbox/footer, enabling and disabling through Save & activate, anonymous post/feed visibility, and preserved artifact archive. Four browser checks passed; temporary post/session were removed and hidden state restored.

### What didn't work

- The initial full suite failed TestConcurrentNewArtifactUpload with race-7 upload 0 returning 409 version_exists instead of the expected 403 not_editor. Three isolated repeats passed, but the next full suite reproduced the failure at race-1. The upload code is outside the modified visibility paths; its existing check-to-publication race is recorded for a separate fix, not hidden by repeated retries.
- The first local browser run failed its readiness assertion because the previous tmux pane had returned to a shell after the old server stopped. A respawn attempt still did not produce a healthy preview. The local preview database also lacked the earlier email feature's 012_email_signins.sql migration; applying that existing local migration and starting a fresh pane produced a healthy server. No new migration is required by the blog flag itself.

### What I learned

Changing a setting used by the React footer also requires invalidating the Session query, not only Settings and Account. Existing saved JSON already has the correct false default without a compatibility adapter.

### What was tricky to build

Published posts must remain previewable by their authorized editors while being hidden from all ordinary visitors and other creators. The preview uses the existing canEditPost policy and private caching rather than a second public route or a user-supplied bypass parameter.

### What warrants a second pair of eyes

Review the post preview checks, RSS SQL predicate and creator-page query guard together. The setting is presentation control, not a promise to revoke cached copies of previously published media URLs.

### What should be done in the future

Resolve the upload authorization race exposed by the full gate, then record final verification. Production deployment remains a separate user decision.

### Code review instructions

Start with settings.Storefront, Admin Settings, pages.go post/journal/creator handlers and feed.go. Run TestJournalHiddenByDefaultAndAdminToggle against a disposable database. Browser evidence is under sources/.

### Technical details

- Existing settings endpoint: PATCH /api/admin/settings with normal admin, CSRF and If-Match requirements.
- Default and missing-property behavior: storefront.journalEnabled=false.
- Local preview: http://localhost:18792; no production settings were changed.

## Step 2: Close the upload authorization race and complete the pre-deployment gate

The blog feature was committed separately as **18c8031 — Hide the public blog by default with an admin setting**. Full-suite validation had repeatedly exposed an existing authorization race in first-artifact upload. I traced and fixed the narrow race rather than repeatedly rerunning tests until they happened to pass.

The complete Go suite now passes, including the concurrent-upload test. A deterministic regression also forces the stale authorization state directly. Build, lint/logging checks and frontend validation are clean; the new setting is implemented and locally qualified without a remote deployment.

### Prompt Context

**User prompt:** See Step 1.
**Assistant interpretation:** Complete the requested pre-deployment change and resolve the concrete failing gate in an isolated follow-up commit.
**Inferred user intent:** Have a reliable artifact-focused site ready before deployment, with an accurate work history.
**Feature commit:** 18c8031.

### What I did

- Traced studioUpload's mayUpload decision and Publisher.CreateDraft's later prepare lookup. If another creator inserted the artifact between them, prepare saw an existing artifact and skipped the creation-conflict branch that normally triggers reauthorization.
- Added the internal RequireNewArtifact upload flag, set from the server's mayUpload result on every attempt. CreateDraft returns the existing ErrArtifactTaken when that decision is stale, and the existing bounded retry performs the actual editor check.
- Kept the fix separate from the visibility setting. No new route, authorization framework, compatibility wrapper or client-provided permission field was added.
- Added TestNewArtifactAuthorizationMustBeRechecked to force an already-existing artifact under a stale new-artifact decision and confirm no release is written.
- Ran the deterministic and concurrent tests together three times, then the full repository suite. Updated ticket status and preserved the final test summary.

### Why

The failure was not merely an assertion timing issue: the stale new-artifact decision could skip the intended editor recheck. Preserving that decision until the publisher's fresh lookup closes the concrete gap while reusing the existing conflict/retry path.

### What worked

- `go test ./pkg/catalog -run 'TestConcurrentNewArtifactUpload|TestNewArtifactAuthorizationMustBeRechecked' -count=3` passed.
- `TEST_DATABASE_URL=... go test ./... -count=1` passed; catalog integration took 24.939s. All database tests used isolated disposable databases.
- `go build ./...` and `make lint logcopter-check` passed with zero lint issues.
- The visibility/admin/browser and frontend checks from Step 1 remain qualified; the follow-up touches only the publisher, upload handler and regression test.

### What didn't work

The first deterministic regression queried Store.Releases with an empty state assuming it meant all states. That method requires a specific state, so it returned zero and the test reported `stale attempt wrote a release: 0 <nil>` in each of three runs. Changed that verification to an explicit total release count for the fixture artifact. The authorization assertion itself already passed; the corrected persistence assertion then passed too.

### What I learned

The first-artifact conflict path already had a correct reauthorization retry, but it only covered conflicts occurring after prepare. A server-owned expectation flag extends that existing path to the earlier lookup window without weakening permission checks or serializing unrelated uploads.

### What was tricky to build

A purely concurrent regression can miss the scheduling window, as the three initial isolated repeats demonstrated. The deterministic test makes the condition explicit and verifies the required error before any new release is stored.

### What warrants a second pair of eyes

Review RequireNewArtifact propagation alongside mayUpload and ErrArtifactTaken handling. The value must remain derived from server authorization, never from an uploaded form field. Existing artifacts still require their normal editor check.

### What should be done in the future

No feature work remains in this ticket. A production deployment is separate; the local preview's storefront.journalEnabled remains false. The recursive catalog API design has been linked to this visibility policy for its later implementation.

### Code review instructions

Review the blog feature commit separately from this three-file upload fix. Reproduce the focused upload tests, the full Go gate and the local admin browser check as needed. The saved browser evidence shows actual enable/disable behavior; final-test-summary.txt records the final passing package results.

### Technical details

- New internal field: releases.Upload.RequireNewArtifact.
- Existing error/retry contract reused: releases.ErrArtifactTaken; no fallback adapter.
- Default public setting: storefront.journalEnabled=false.
- No remote infrastructure or production settings were changed.

## Step 3: Open the integration PR and reconcile squash-merged main

Opened PR #10 for the completed branch and requested Codex review. GitHub initially reported merge conflicts because main contains older squash-merged versions of work retained in this branch's ancestry. Merging main and retaining the newer branch versions in the five conflicting files produced no runtime diff from the already validated HEAD.

### Prompt Context

**User prompt (verbatim):** "ok, PR, wait for code review,"

**Assistant interpretation:** Publish the branch, request review and monitor it without merging or deploying.

**Inferred user intent:** Review the completed integration before production changes.

### Evidence and commits

- PR: https://github.com/go-go-golems/francois/pull/10
- Codex request: https://github.com/go-go-golems/francois/pull/10#issuecomment-6241390753
- `git merge origin/main` initially conflicted in pages.go, storefront_test.go, catalog.js, layout.tmpl and instances.tsx. Inspected combined diffs; main's conflicting alternatives were older download labels, launch handling and unconditional public script inclusion.
- Retained the newer feature branch implementations; `git diff HEAD --stat` was empty after resolution. Main's nonconflicting changes were already present.
- Node 24/Corepack `pnpm check` and `git diff --check` passed after resolution. The prior full validation remains applicable because the runtime tree is identical.

### Noteworthy decisions or failures

Kept the branch's completed features together as requested. The PR description explicitly distinguishes implemented changes from the two design-only tickets. Used a merge commit to reconcile ancestry rather than rewriting published history.

### Remaining requirements and next action

Wait for GitHub checks and Codex review. No merge or deployment is authorized by this request.

## Step 4: Receive the requested code review

Codex completed review of e27cfe5 and returned two P2 findings: the shared React shell lacks a sign-out action, and Account's title effect does not restore the prior document title on route departure. These are recorded for the next implementation step; this request was to open the PR and wait for review.

### Prompt Context

See Step 3 for the verbatim request.

### Evidence and commits

- `gh api repos/go-go-golems/francois/pulls/10/comments` returned findings in ui/catalog/ui.tsx:115 and ui/catalog/account.tsx:12.
- GitHub CI passed tests, frontend checks, lint, CodeQL, dependency review, Go vulnerability checks, GoSec and image publication. The GitOps deployment step was skipped.
- TruffleHog failed with one unverified URI and zero verified secrets. Its annotation points to pkg/catalog/identity/email_test.go:126, the deliberately invalid issuer containing dummy user/password userinfo. This remains an outstanding CI finding, not a leaked production credential.

### Noteworthy decisions or failures

No merge or deployment occurred. Review findings are not marked resolved, and CI is not represented as fully passing.

### Remaining requirements and next action

Address the two P2 comments and the scanner's test-fixture finding before considering merge. PR #10 remains open.

## Step 5: Fix shared sign-out and Account title cleanup

Addressed both P2 review findings with a shared SignOut component and effect cleanup. The shared footer now exposes logout on Instances, Studio and Admin; Account uses the same component instead of duplicating its mutation and error handling. Leaving Account restores the title captured on entry.

### Prompt Context

**User prompt (verbatim):** "address,"

**Assistant interpretation:** Implement the review fixes, validate and push them for another review.

**Inferred user intent:** Resolve the open review comments before merge.

### What I did

- Added SignOut in ui/catalog/ui.tsx, reused in the footer and Account. It posts to the existing /auth/logout endpoint, disables while pending, follows the existing optional provider redirect and displays failures through the existing notice.
- Added cleanup to Account's title effect, matching the launcher's existing lifetime pattern.
- Rebuilt the tracked catalog.js bundle.
- Added scripts/02-verify-review.mjs and sources/review-browser-validation.json. Its temporary local administrator session is revoked in finally; no credential is persisted.

### Why

A single small component keeps the two logout entry points consistent. The title belongs to Account's mounted lifetime, so restoring the prior value requires no central router redesign.

### What worked

- Node 24/Corepack pnpm check, pnpm build and all nine pnpm tests passed.
- Real Chromium checks passed for Account → Studio and Account → Admin title restoration, footer logout visibility, a simulated logout failure retaining the session, successful actual logout revoking the session and returning home, and anonymous shell hiding logout.
- Restarted only the owned local port 18792 using lsof-who and tmux session francois-review-010.

### What didn't work

The first browser harness intercepted **/api/auth/logout, but the existing endpoint is /auth/logout. The actual logout therefore succeeded instead of receiving the simulated failure; getByRole('status').filter({hasText:'Temporary logout failure'}) timed out after 30000ms. Corrected the interception and reran all five checks successfully.

TruffleHog's existing CI failure remains a known false positive in the historical test fixture from commit 2f2fe66: the deliberately invalid issuer uses dummy user/password userinfo. Inspected the complete file and its sole feature-branch history entry. The scanner reads historical additions; a current-line annotation would not suppress the old finding. Did not disable URI detection, exempt the whole test file or rewrite published commits. My initial progress message promised to clear that failure; the supported narrow inline suppression cannot clear historical bytes, so that part remains outstanding rather than weakening the check.

### What I learned

The frontend mutation uses the auth endpoint directly, outside /api. Testing the error branch must intercept that exact route while allowing the final success case to reach the real server.

### What was tricky to build

The shared footer stays mounted across SPA navigation, while Account unmounts. Cleanup must restore the captured title before another page's effects apply their own title.

### What warrants a second pair of eyes

Verify footer logout matches the Account behavior and title cleanup remains correct across repeated Account visits. Treat the historical scanner finding explicitly as unresolved CI rather than claiming a fully green PR.

### What should be done in the future

Receive another review of these fixes. Resolve or explicitly adjudicate the historical test-data scanner finding before merge. No production changes are part of this step.

### Code review instructions

Review ui/catalog/ui.tsx SignOut and Footer, and Account's title effect. Reproduce the local browser harness against the owned preview; its failure branch is simulated but session persistence and revocation are verified against the actual local backend.

### Technical details

The fixture browser session lasts at most 15 minutes and is revoked on exit. Error-path simulation returns HTTP 500 once; the subsequent logout request reaches the real server. No new endpoint, dependency or environment-variable read is introduced.
