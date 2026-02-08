/**
 * Generates PWA icon PNGs from an SVG source.
 * 
 * Usage: node scripts/generate-icons.js
 * 
 * This creates placeholder PNGs in public/ at the sizes needed for PWA install.
 * For production, replace these with proper designed icons.
 * 
 * Since we can't rasterize SVG to PNG without a dependency like sharp/canvas,
 * this generates simple solid-color PNGs with embedded metadata that browsers
 * accept for PWA install prompts.
 */

import { writeFileSync } from 'node:fs'

// Minimal valid PNG generator (solid color with basic structure)
function createPng(size, bgR, bgG, bgB) {
  // PNG file structure: signature + IHDR + IDAT + IEND
  const signature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10])

  // IHDR chunk
  const ihdrData = Buffer.alloc(13)
  ihdrData.writeUInt32BE(size, 0)   // width
  ihdrData.writeUInt32BE(size, 4)   // height
  ihdrData[8] = 8                    // bit depth
  ihdrData[9] = 2                    // color type: RGB
  ihdrData[10] = 0                   // compression
  ihdrData[11] = 0                   // filter
  ihdrData[12] = 0                   // interlace
  const ihdr = createChunk('IHDR', ihdrData)

  // IDAT: raw image data (uncompressed for simplicity)
  // Each row: filter byte (0) + RGB pixels
  const rowLen = 1 + size * 3
  const rawData = Buffer.alloc(rowLen * size)
  
  // Draw a simple icon: dark background with a blue-ish rectangle and circle
  for (let y = 0; y < size; y++) {
    const rowOffset = y * rowLen
    rawData[rowOffset] = 0 // no filter
    for (let x = 0; x < size; x++) {
      const px = rowOffset + 1 + x * 3
      
      // Rounded rectangle border area (icon shape)
      const margin = Math.floor(size * 0.12)
      const radius = Math.floor(size * 0.15)
      const inRect = x >= margin && x < size - margin && y >= margin && y < size - margin
      
      // Check if we're in the rounded corner exclusion zone
      let inRounded = inRect
      if (inRect) {
        // Top-left corner
        if (x < margin + radius && y < margin + radius) {
          const dx = x - (margin + radius)
          const dy = y - (margin + radius)
          inRounded = dx * dx + dy * dy <= radius * radius
        }
        // Top-right
        if (x >= size - margin - radius && y < margin + radius) {
          const dx = x - (size - margin - radius)
          const dy = y - (margin + radius)
          inRounded = dx * dx + dy * dy <= radius * radius
        }
        // Bottom-left
        if (x < margin + radius && y >= size - margin - radius) {
          const dx = x - (margin + radius)
          const dy = y - (size - margin - radius)
          inRounded = dx * dx + dy * dy <= radius * radius
        }
        // Bottom-right
        if (x >= size - margin - radius && y >= size - margin - radius) {
          const dx = x - (size - margin - radius)
          const dy = y - (size - margin - radius)
          inRounded = dx * dx + dy * dy <= radius * radius
        }
      }

      if (inRounded) {
        // Inside the icon shape
        // Draw a small "sun" circle in top-left area
        const sunCx = Math.floor(size * 0.33)
        const sunCy = Math.floor(size * 0.35)
        const sunR = Math.floor(size * 0.07)
        const sdx = x - sunCx
        const sdy = y - sunCy
        const inSun = sdx * sdx + sdy * sdy <= sunR * sunR

        // Draw a "mountain" triangle in bottom half
        const mountainPeak = Math.floor(size * 0.45)
        const mountainBase = Math.floor(size * 0.82)
        const mountainLeft = Math.floor(size * 0.25)
        const mountainRight = Math.floor(size * 0.75)
        const mountainMid = Math.floor((mountainLeft + mountainRight) / 2)
        let inMountain = false
        if (y >= mountainPeak && y <= mountainBase) {
          const progress = (y - mountainPeak) / (mountainBase - mountainPeak)
          const halfWidth = progress * (mountainRight - mountainLeft) / 2
          inMountain = x >= mountainMid - halfWidth && x <= mountainMid + halfWidth
        }

        if (inSun) {
          // Golden sun
          rawData[px] = 250
          rawData[px + 1] = 204
          rawData[px + 2] = 21
        } else if (inMountain) {
          // Blue accent mountain
          rawData[px] = 59
          rawData[px + 1] = 130
          rawData[px + 2] = 246
        } else {
          // Dark background inside icon
          rawData[px] = 20
          rawData[px + 1] = 20
          rawData[px + 2] = 20
        }
      } else {
        // Outside = transparent-ish (or just match bg)
        rawData[px] = bgR
        rawData[px + 1] = bgG
        rawData[px + 2] = bgB
      }
    }
  }

  // Compress with zlib (deflate)
  const { deflateSync } = await import('node:zlib')
  const compressed = deflateSync(rawData)
  const idat = createChunk('IDAT', compressed)

  // IEND
  const iend = createChunk('IEND', Buffer.alloc(0))

  return Buffer.concat([signature, ihdr, idat, iend])
}

function createChunk(type, data) {
  const len = Buffer.alloc(4)
  len.writeUInt32BE(data.length, 0)
  const typeBuffer = Buffer.from(type, 'ascii')
  const crcData = Buffer.concat([typeBuffer, data])
  const crc = Buffer.alloc(4)
  crc.writeUInt32BE(crc32(crcData), 0)
  return Buffer.concat([len, typeBuffer, data, crc])
}

// CRC32 for PNG chunks
function crc32(buf) {
  let crc = 0xFFFFFFFF
  for (let i = 0; i < buf.length; i++) {
    crc ^= buf[i]
    for (let j = 0; j < 8; j++) {
      crc = (crc >>> 1) ^ (crc & 1 ? 0xEDB88320 : 0)
    }
  }
  return (crc ^ 0xFFFFFFFF) >>> 0
}

// Generate icons
const sizes = [192, 512]
for (const size of sizes) {
  const png = await createPng(size, 10, 10, 10)
  const path = `public/pwa-${size}x${size}.png`
  writeFileSync(path, png)
  console.log(`Generated ${path} (${png.length} bytes)`)
}

console.log('Done! Replace these with properly designed icons for production.')
