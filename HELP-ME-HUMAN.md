# Assets I Need You to Create

## PWA Icons (required for install-to-home-screen)

These go in `ui/public/`.

| File | Size | Format | Notes |
|------|------|--------|-------|
| `ui/public/pwa-192x192.png` | 192x192 px | PNG, transparent background | App icon on home screen. Should be the Photog logo/icon. |
| `ui/public/pwa-512x512.png` | 512x512 px | PNG, transparent background | Used by Android splash screen and larger displays. Same design as the 192, just bigger. |

## PWA Screenshots (optional, but makes the install prompt look much nicer on Android)

These also go in `ui/public/`.

| File | Size | Format | Notes |
|------|------|--------|-------|
| `ui/public/screenshot-wide.png` | 1280x720 px | PNG or JPG | A desktop-width screenshot of the app showing the photo timeline. |
| `ui/public/screenshot-narrow.png` | 390x844 px | PNG or JPG | A phone-width screenshot of the same. |

If you don't want to bother with screenshots, I can remove them from the manifest. The icons are the important part.

## CasaOS App Store Assets (only needed if you submit to the store)

These go in a `casaos/` folder at the project root.

| File | Size | Format | Notes |
|------|------|--------|-------|
| `casaos/icon.png` | 192x192 px | PNG, transparent background | Can be the same file as the PWA 192 icon. |
| `casaos/screenshot-1.png` | 1280x720 px | PNG or JPG | Required. At least one screenshot showing the app running. |
| `casaos/thumbnail.png` | 784x442 px | PNG, transparent background, rounded corners | Only needed if you want Photog featured on the store front page. |

## Summary

The bare minimum to get PWA install working is just the two icon PNGs. Everything else is polish.

```
ui/public/
  pwa-192x192.png    <-- you make this
  pwa-512x512.png    <-- you make this
  screenshot-wide.png    <-- optional
  screenshot-narrow.png  <-- optional

casaos/
  icon.png           <-- only if submitting to store
  screenshot-1.png   <-- only if submitting to store
  thumbnail.png      <-- only if submitting to store
```
