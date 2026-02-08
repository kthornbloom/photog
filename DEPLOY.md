# Installing Photog on CasaOS

## Before you start

You need a Docker image available for CasaOS to pull. The easiest way is to build and push it to Docker Hub (or GitHub Container Registry) from any machine with Docker:

```bash
git clone https://github.com/YOUR_USER/photog.git
cd photog
docker build -t YOUR_DOCKERHUB_USER/photog:0.1.0 .
docker push YOUR_DOCKERHUB_USER/photog:0.1.0
```

Replace `YOUR_DOCKERHUB_USER` with your actual Docker Hub username. Use a specific version tag (not `:latest`) -- CasaOS recommends this and the app store requires it.

---

## Installing via "Custom App"

1. Open your CasaOS dashboard in a browser
2. Click the **+** button, then **Install a customized app**
3. Fill in the form:

**Docker Image**
```
YOUR_DOCKERHUB_USER/photog:0.1.0
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

When you've made changes to Photog and want to deploy them:

1. On your dev machine, rebuild and push with a new version tag:
   ```bash
   docker build -t YOUR_DOCKERHUB_USER/photog:0.2.0 .
   docker push YOUR_DOCKERHUB_USER/photog:0.2.0
   ```

2. On CasaOS, go to the Photog app settings and update the image tag to the new version (e.g. `YOUR_DOCKERHUB_USER/photog:0.2.0`), then click **Save**. CasaOS pulls the new image and restarts the container. Your cache and database carry over -- no re-indexing needed unless the database schema changed.

That's it. Your photo library volume and cache volume stay exactly where they are.

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
