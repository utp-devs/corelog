# E2E defects found by the P1/P2 Playwright round

Generated from the run of the extended suite (desktop 1440x900, tablet 820x1180, mobile 390x844).
Evidence paths are under `ui/test-results/` (git-ignored) and are regenerated on every run.
`known-violations.json` now records the remaining layout-audit findings per screen|viewport so that
the suite is green. The goal of the next round is to empty that file.

## Defects

| Id | Screen | Viewport | Rule | Evidence | Suggested fix |
| --- | --- | --- | --- | --- | --- |
| D-01 | `/login` (S-01) | desktop, tablet | width, dead space (split layout instead of the centered auth card) | `test-results/specs-auth-login-pantalla--695cb-mple-la-auditoria-de-layout-desktop/test-failed-1.png` (audit attachment) | Decide S-01 vs the split brand panel. If S-01 wins, center the 400-440 px card; otherwise update S-01 and the audit template to `split`. Tighten the baseline after the decision. |
| D-02 | `/teams/:id` (S-26) | all | route missing: `ui/src/app/router.tsx` has only `/teams`, so team cards link to a 404 page. `TeamPage` is not routed | Team card links (`TeamCard.tsx` line 22) | Add `{ path: 'teams/:id', element: <TeamPage/> }` under the org shell. Blocks 2 fixme tests in `specs/org/teams.spec.ts`. |
| D-03 | Top bar, every page (S-00 shell) | tablet 820 px | overlap, clipped text: the "Crear" button covers "Paneles" and "Equipos", the search box squeezes the nav, and the organization switcher is pushed out of view | `test-results/specs-shell-shell-navegaci-f3d01-stra-la-organizacion-activa-tablet/test-failed-1.png` | Collapse the top menus into the drawer or the "more" menu below ~1024 px, keep the switcher visible. Blocks 2 tablet fixme tests in `specs/shell/shell.spec.ts`. |
| D-04 | Top bar, every page | mobile 390 px | clipped control: the user avatar (menu trigger) is cut at the right edge; the search box crowds the bar | `test-results/journeys-smoke-all-screens-2a3e0-as-pantallas-de-owner-smoke-mobile/owner-mobile-people-chromium.png` | Hide the search field behind an icon on phones, keep the avatar fully inside the bar. |
| D-05 | `/profile/general` (S-27 profile) | mobile | page padding: the "General" heading and the tab row start at x=0 (no gutter) | `test-results/journeys-smoke-all-screens-2a3e0-as-pantallas-de-owner-smoke-mobile/owner-mobile-profile-general-chromium.png` | Apply the page gutter (`--cl-space-*`) to the profile layout on narrow widths. |
| D-06 | Every page that shows the session check or a Spin | all | console error: `Warning: [antd: Spin] tip is deprecated`, source `ui/src/adapters/in/ui/common/ProtectedRoute.tsx:15` (and other `Spin` with `tip`) | Baseline rule "console errors" on all 20 screens | Replace `tip=` with `description=` (antd 6). This alone clears the hygiene rule on most screens. |
| D-07 | `/dashboard` (S-10) | desktop, tablet, mobile | overlap: empty-state texts "Crea tu primer proyecto", "Crea una incidencia con el botón" overlap each other | `test-results/journeys-smoke-all-screens-2a3e0-as-pantallas-de-owner-smoke-desktop/owner-desktop-dashboard-chromium.png` | Give the empty-state copy its own block with spacing; stop the inline spans from wrapping into each other. |
| D-08 | `/issues` (S-52) | tablet | dead space: the table header block is about 190 px tall, a grey gap above the column titles | `test-results/specs-shell-shell-navegaci-f3d01-stra-la-organizacion-activa-tablet/test-failed-1.png` | Fix the sticky header height on the table at 820 px. |
| D-09 | `/people/:id` (S-27 ficha) | desktop | width (page narrower than 85 % at 1440 px) and dead space | `test-results/journeys-smoke-all-screens-2a3e0-as-pantallas-de-owner-smoke-desktop/owner-desktop-person-chromium.png` | Use the split template width (fill the content area) or the reading width consistently. |
| D-10 | `/people`, `/teams`, `/projects`, `/organization/members` | desktop, tablet, mobile | dead space in the lower half of short lists (no empty state, large unused area) | Baseline rule "dead space" on these screens | Use the wide template fill rule, or grow the list area with a fixed min-height. |
| D-11 | `/issues/new` (S-51) | desktop | dead space: 69 % of the visible area is empty for the requester role (form uses one column) | `test-results/journeys-smoke-all-screens-*-requester*` (requester desktop) | Use the two-column grid of the reading template for the form, or the side preview. |
| D-12 | `/profile`, `/profile/general`, `/organization/settings`, `/organization/invitations`, `/issues`, `/issue-detail`, `/project-board` | tablet and mobile | clipped text (labels and buttons cut by their container) | Baseline rule "clipped text" | Allow wrapping for labels and tags, or add `title` plus ellipsis per the audit rule. |
| D-13 | Mobile across all screens | mobile | tap targets below 32 px (icon buttons in tables, toolbar buttons) | Baseline rule "tap targets" | Raise the hit area of icon buttons to 32 px on the mobile breakpoint. |
| D-14 | `/issues`, `/project-backlog`, `/project-board`, `/project-sprints`, `/project-calendar`, `/issue-detail` | mobile | horizontal overflow and overlap (toolbar or table wider than 390 px) | Baseline rules "horizontal overflow", "overlap", "clipped control" | Move the toolbar into a wrapping row or a drawer on narrow widths; keep tables in `data-scroll-x` containers. |

## Data and test-harness notes (not UI defects)

- N-01: deactivated accounts created by the e2e suite stay listed in the Acme directory
  (`/people`), because deactivation does not remove memberships. The directory fills with
  `e2e.*@corelog.test` users. Options: remove the membership when the test deactivates the user,
  or filter deactivated users out of the directory.
- N-02: closed invitations (accepted or revoked) accumulate in Acme, since there is no hard delete
  for invitations. Acceptable for now; consider a retention purge in the seed.
- N-03: the `expired` invitation state is skipped: no API can backdate `expires_at`. Backend tests
  cover it.

## Fixme tests (blocked by a defect)

- `specs/org/teams.spec.ts`: "member agrega a una persona al equipo" and "pestanas de la pagina de
  equipo" (D-02).
- `specs/shell/shell.spec.ts`: "el selector de organizacion muestra la organizacion activa" and
  "cambiar de organizacion persiste la seleccion tras recargar" are `test.fixme` on the tablet
  project only (D-03). They run on desktop.

## Skips by design

- Mobile (below 768 px): the organization switcher is in the drawer, so the top-bar tests skip there.
- Mobile: owner-only row actions on the invitations table are listed as cards; the row-action tests skip there.
- `@visual` tests skip outside Linux (docs/11-testing-strategy.md section 3.2).

## Screenshot review (new screens)

- Organization settings (desktop): fills the reading column, policies and the section nav align; the
  lower third is empty (acceptable for reading). No clipping.
- People directory (desktop): list is readable; it shows the test-account pollution (N-01).
- Profile general (desktop): clean two-level layout, email disabled with hint. No issues.
- Teams (desktop): two cards, equal heights; the lower area is empty (D-10).
- Onboarding step 1 (desktop): two equal cards, stepper aligned. The join panel's button is visually
  detached from its input; minor.
- Profile general (mobile): content is flush to the left edge (D-05).
- People directory (mobile): rows readable; the top bar crowds the avatar (D-04).
