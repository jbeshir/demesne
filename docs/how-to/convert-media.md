# Convert and manipulate media in a sandbox

When you need to transcode video, convert or resize images, or process audio, ask your agent to do the work in a demesne sandbox on the `media` image. It carries a broad media toolbox and runs entirely on disk, so it needs no network access — useful for format conversion, thumbnail and frame extraction, image optimization, and audio re-encoding over your own files.

## The image

demesne builds the `media` image locally the first time it's used — from Ubuntu 24.04 plus the tools below — and caches it, so only the first run is slow. The cache is shared between the host and any nested sandboxes, so an agent running a larger pipeline can convert media in a child sandbox too. Nothing in the toolbox needs the internet, so this work runs with egress disabled.

## What's installed

| Area | Tools |
|------|-------|
| Video / audio transcode | `ffmpeg` — broad codec coverage (H.264/x264, H.265/HEVC, VP8/9, AV1 encode + decode, Opus, MP3, WebP, JPEG XL) |
| Image manipulation | ImageMagick 6 (`convert`, `mogrify`, `identify`), `vips` (libvips), WebP tools (`cwebp`/`dwebp`/`gif2webp`), `rsvg-convert` (SVG), `potrace`, `gifsicle` |
| Image optimization | `jpegoptim`, `pngquant`, `optipng` |
| Audio | `sox` (all formats), `lame`, `flac`, `opus-tools` — and ffmpeg |
| Inspection / metadata | `mediainfo`, `exiftool`, `ghostscript` (PDF/EPS rasterization) |

ffmpeg's native AAC encoder handles AAC; the non-free libfdk-aac is not included. Hardware-accelerated encoders (NVENC, VAAPI, QSV) are out of scope — the image is CPU-only, built for processing mounted files.

## What you can ask for

Point your agent at the files you want converted and describe the result you want; it mounts them and hands the output back. Some examples, with the command each maps to so you know the toolbox can do it:

- "Transcode this video to MP4" → `ffmpeg -i input.mov -c:v libx264 -crf 23 -c:a aac output.mp4`
- "Pull a thumbnail frame out of this clip" → `ffmpeg -i input.mp4 -ss 5 -frames:v 1 thumb.png`
- "Resize these photos to 800px wide" → `vips thumbnail photo.jpg photo-thumb.jpg 800`
- "Convert this PNG to an optimized WebP" → `cwebp -q 80 photo.png -o photo.webp`
- "Extract the audio as Opus" → `ffmpeg -i clip.mp4 -vn -c:a libopus clip.opus`
- "Tell me about this file" → `mediainfo clip.mp4` or `ffprobe clip.mp4`

To check the toolbox is working without any input files, ask for a synthesized test frame: ffmpeg can generate one (`ffmpeg -f lavfi -i testsrc=duration=1:size=320x240:rate=1 -frames:v 1 frame.png`) and ImageMagick's `identify` can inspect it.

## See also

- [`sandbox_script` reference](../reference/tools/sandbox_script.md)
- [Configuration reference](../reference/configuration.md)
