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

For CasaOS, use `:latest` for the simplest setup. Pair with Watchtower for automatic updates (see "Automatic updates" below).

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

> **Note:** CasaOS's built-in "Check then update" button does **not** reliably detect changes to the `:latest` tag for custom-installed apps. This is a known CasaOS limitation. Use one of the methods below instead.

### Automatic updates with Watchtower (recommended)

[Watchtower](https://containrrr.dev/watchtower/) is a container that monitors your running Docker containers and automatically updates them when new images are available. Install it **once** and it handles updates for Photog (and any other containers you opt in) forever.

**Install Watchtower as a separate CasaOS app:**

1. Click **+** > **Install a customized app** in CasaOS
2. Fill in:
   - **Docker Image:** `containrrr/watchtower:latest`
   - **Title:** Watchtower
   - **Network:** bridge
   - **Volumes:** Add one mapping:

     | Host | Container |
     |------|-----------|
     | `/var/run/docker.sock` | `/var/run/docker.sock` |

   - **Environment Variables:**

     | Name | Value | Purpose |
     |------|-------|---------|
     | `WATCHTOWER_CLEANUP` | `true` | Remove old images after updating |
     | `WATCHTOWER_POLL_INTERVAL` | `900` | Check every 15 minutes (in seconds) |
     | `WATCHTOWER_LABEL_ENABLE` | `true` | Only update containers with the opt-in label |

3. Click **Install**

The `WATCHTOWER_LABEL_ENABLE=true` setting means Watchtower will **only** update containers that have the `com.centurylinklabs.watchtower.enable=true` label. Photog's compose file already includes this label, so when you install Photog via CasaOS using this compose file, it will be automatically opted in.

> **Important:** When installing Photog on CasaOS, you need to make sure the Watchtower label is present. In the CasaOS custom app form, there is no UI for Docker labels. If CasaOS does not carry over the label from the compose file, you can alternatively set `WATCHTOWER_LABEL_ENABLE` to `false` on Watchtower -- but be aware it will then monitor **all** containers on the host.

When a new image is detected, Watchtower:
1. Pulls the new image
2. Gracefully stops the Photog container
3. Restarts it with the new image (same settings/volumes)
4. Removes the old image to save disk space

Your cache and database carry over -- no re-indexing needed unless the database schema changed.

### Manual updates via SSH

If you prefer to update manually:

```bash
# Pull the latest image and restart
docker pull ghcr.io/kthornbloom/photog:latest
# Find and restart the Photog container
docker restart $(docker ps -q --filter "ancestor=ghcr.io/kthornbloom/photog")
```

Or more reliably, stop and recreate:
```bash
# Find the container name
docker ps --filter "ancestor=ghcr.io/kthornbloom/photog" --format "{{.Names}}"
# Then stop, remove, and let CasaOS recreate it (or recreate manually)
```

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
