Build a complete, production-ready, self-hosted photo viewer app optimized for CasaOS (Docker-first) with these exact requirements:
Core Requirements

Read-only mounted volumes for huge existing photo/video libraries (multiple folders ok, originals never modified)
Aggressive caching + thumbnails (disk cache in persistent volume, HTTP immutable cache headers, browser/service-worker caching)
Metadata-first sorting: EXIF/XMP date-taken primary, file-modified fallback → reliable vertical timeline grouped by date (exactly like Google Photos: month/year sticky headers, infinite virtual scroll)
Native video playback (HTML5 <video> with poster thumbnails, range-request streaming support)
Initial indexing/optimization phase acceptable (scan once, store index in SQLite)
Runs blazing fast on low-power hardware (Raspberry Pi, old NUC, etc.) → prioritize Go or very lightweight Node
Fully installable as PWA on iOS/Android (manifest + service worker that caches thumbnails aggressively)

Preferred Tech Stack (best balance of speed + Vue 3 customizability)

Backend: Go (single binary, embedded frontend) – same architecture as Photofield because it already solves 95% of the hard parts perfectly (fast indexing ~1000–10k files/sec, SQLite cache, thumbnail system with libjpeg-turbo + embedded thumbs, read-only volumes, video support, timeline layout)
Frontend: Pure Vue 3 + Vite + Tailwind CSS + Headless UI (no BalmUI). Replace/rebuild the entire UI layer from scratch for maximum custom control.
PWA: vite-plugin-pwa (offline thumbnail cache, install prompt)
Virtual scrolling: TanStack Vue Virtual or vue-virtual-scroller for the infinite vertical timeline
Image serving: Go handlers with ETag + Cache-Control: immutable, long max-age
Thumbnails: Generate on-demand with github.com/h2non/imaging or libvips bindings, WebP output, store in /cache/thumbs persistent volume
Database: SQLite (photo index: path, taken_at, width, height, orientation, type)

UI / UX

Single page: vertical infinite-scroll timeline grouped by date (year/month headers that stick)
Masonry/grid layout per day or month (responsive columns)
Tap photo → full-screen viewer (zoom/pan like Photofield or Google Photos, swipe between photos)
Video auto-plays muted on hover in grid, full controls in viewer
Search/filter by date range optional but nice
Dark mode by default, highly themeable

CasaOS / Docker

Provide ready-to-use docker-compose.yml + Dockerfile (single container preferred, embed Vue build into Go binary like Photofield does)
Volumes: /photos:ro (libraries), /cache (thumbnails + sqlite)
Port 8080, simple config via env or config.yaml for photo paths

Development Workflow

Frontend dev: npm run dev with hot reload (proxy to Go backend)
Production: build Vue → embed into Go binary → single static container
Make the Vue part completely independent and easy to restyle/replace later

Starting Point Recommendation (strongly preferred)
Fork https://github.com/SmilyOrg/photofield (it already has Go backend + Vue 3 + timeline + everything performance-critical) and:

Delete/replace the ui/ folder with a clean Vite + Tailwind + TanStack Virtual project
Add PWA plugin
Rebuild & re-embed the frontend
Keep backend 100% unchanged (or only minor tweaks)

If you prefer building from scratch, use the same architecture but start with a minimal Go server + SQLite indexer.
Output the full project (or key files + patch instructions) so I can immediately git clone, build, and deploy to CasaOS.