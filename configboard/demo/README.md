# demo

| File | What it is |
| --- | --- |
| `configboard-demo.mp4` | 3 min 11 s silent walkthrough, 1920×1350, captions burned in |
| `TRANSCRIPT.md` | The same walkthrough as text, timestamped — read this if you can't watch |
| `manifest.tsv` | One line per frame: PNG filename, seconds on screen, caption |
| `encode.swift` | Turns the frames + manifest into the `.mp4` |

The video has **no audio**. The burned-in captions are the narration; `TRANSCRIPT.md`
is the longer form of the same script.

## Re-recording

The frames are not checked in — they are ~1 MB each and regenerable. To re-record:

1. Run the app (`cd app && npm run dev`) against an organization with real data.
2. Sign in **before** you start capturing. Nothing in the shipped frames shows an
   account or organization identity, and it should stay that way.
3. Capture 1500×950 viewport screenshots in the order `manifest.tsv` lists them.
4. Edit the captions in `manifest.tsv` to match what you captured.
5. Encode:

   ```
   swift encode.swift manifest.tsv configboard-demo.mp4
   ```

`encode.swift` needs only macOS — AVFoundation and AppKit, no ffmpeg, no ImageMagick.
It scales each PNG to 1920 wide, draws a caption band underneath, holds each frame for
its listed duration at 10 fps, and writes H.264 in an MP4 container. Frame dimensions
are assumed to be 3000×1882 (a 1500×950 viewport on a 2× display); other sizes will
letterbox or stretch, so adjust `imageH` if you capture at a different aspect ratio.
