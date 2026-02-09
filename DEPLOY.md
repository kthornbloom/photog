# Installing Photog on CasaOS

## Before you start

Every push to `main`/`master` automatically builds a Docker image and publishes it to the GitHub Container Registry (GHCR) via GitHub Actions. You don't need to build or push anything manually.

The image is available at:

```
ghcr.io/kthornbloom/photog
```

### Available tags

Every push to `master` automatically creates an incrementing version tag (`v0.0.1`, `v0.0.2`, `v0.0.3`, ...) and pushes Docker images with matching tags.

| Tag | When it's created | Example |
|-----|-------------------|---------|
| `latest` | Every push to the default branch | `ghcr.io/kthornbloom/photog:latest` |
| `v0.0.X` | Every push (auto-incrementing) | `ghcr.io/kthornbloom/photog:v0.0.5` |
| `sha-abc1234` | Every push (short commit SHA) | `ghcr.io/kthornbloom/photog:sha-a1b2c3d` |
| `1.0.0`, `1.0` | When you manually push a `v1.0.0` git tag | `ghcr.io/kthornbloom/photog:1.0.0` |

For CasaOS, use `:latest` for the simplest setup. The compose file includes a Watchtower sidecar that automatically detects and pulls new images (see "Automatic updates" below).

---

## Installing via "Custom App"

1. Open your CasaOS dashboard in a browser
2. Click the **+** button, then **Install a customized app**
3. Fill in the form:

**Docker Image**
```
ghcr.io/kthornbloom/photog:latest
```

**App Name:** Photog

**Icon URL** (optional): any square PNG/SVG URL you'd like for the dashboard tile

**Network - Port:** Add one mapping:

| Host | Container | Protocol |
|------|-----------|----------|
| 8080 | 8080      | TCP      |

**Volumes:** Add two entries:

| Host path | Container path | Notes |
|-----------|----------------|-------|
| The path to your photos on the CasaOS machine, e.g. `/DATA/Photos` | `/photos` | Must be the real path where your library lives |
| `/DATA/AppData/photog/cache` | `/cache` | Photog stores its database and thumbnails here |

The photos volume is read-only by design. Photog will never modify, move, or delete your originals. If your photos are spread across multiple folders (e.g. a camera roll and a separate archive), add a volume entry for each one and map them to different paths inside the container:

| Host path | Container path |
|-----------|----------------|
| `/DATA/Photos/CameraRoll` | `/photos/camera` |
| `/DATA/Photos/Archive` | `/photos/archive` |

Then add this environment variable so Photog knows about both:

**Environment Variables:**

| Name | Value |
|------|-------|
| `PHOTOG_PHOTO_PATHS` | `/photos/camera,/photos/archive` |

If you only have one photos folder mounted at `/photos`, you can skip this -- the default already points there.

4. Click **Install**

It pulls the image and starts the container. Open Photog from your CasaOS dashboard or go to `http://your-casaos-ip:8080`.

---

## What happens on first launch

Photog scans your entire photo library and builds an index. You'll see a progress bar at the top of the screen. The timeline populates as it goes, so you can start browsing immediately.

Thumbnails are created the first time you scroll past each photo (not upfront). The very first scroll through new photos will feel slightly slower. After that, thumbnails are cached to disk and everything loads instantly.

All of this data lives in the `/cache` volume. If you ever delete that volume, Photog just rebuilds everything on next start. Nothing is lost.

---

## Updating to a new version

Just push your changes to `main`/`master` (or tag a new release). GitHub Actions builds and pushes the new image to GHCR automatically.

### Automatic updates (Watchtower)

The compose file includes a [Watchtower](https://containrrr.dev/watchtower/) sidecar container that automatically checks the registry for new images every hour (configurable via `WATCHTOWER_POLL_INTERVAL` in seconds). When a new image is detected, Watchtower:

1. Pulls the new image
2. Gracefully stops the running Photog container
3. Restarts it with the new image (same settings/volumes)
4. Removes the old image to save disk space

No manual intervention required. Your cache and database carry over -- no re-indexing needed unless the database schema changed.

### Manual updates

If you prefer to update manually, or don't want to wait for the next poll:

1. SSH into your CasaOS machine
2. Run: `docker compose -f /path/to/docker-compose.yml pull && docker compose -f /path/to/docker-compose.yml up -d`

Or simply restart the Watchtower container from CasaOS to trigger an immediate check.

> **Note:** CasaOS's built-in "Check then update" button does not reliably detect changes to the `:latest` tag for custom-installed apps. This is a known CasaOS limitation, which is why we bundle Watchtower.

---

## Picking up new photos

Photog re-scans your library every time the container starts. Files already in the database are skipped, so restarts are fast. Only new files get indexed.

You can also trigger a re-scan without restarting by visiting:
```
http://your-casaos-ip:8080/api/index
```
and sending a POST request (or just restart the container from CasaOS).

---

## If something isn't working

- **"No photos yet" on screen:** Your volume path is probably wrong. Go back into the app settings on CasaOS and make sure the host path actually contains your photos. Check with `ls /DATA/Photos` (or wherever you pointed it) via SSH.
- **Photos appear in the wrong order:** Photog uses the EXIF "date taken" when available, and falls back to file modification date. If you copied files in bulk, the mod dates may all be the same -- that's a source data issue, not a Photog bug.
- **Want to start completely fresh:** Stop the app, delete the `/DATA/AppData/photog/cache` folder, and start the app again. It re-indexes and regenerates all thumbnails from scratch.
