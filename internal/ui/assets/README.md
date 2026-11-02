# Header logo

The current main-window wordmark is `logo-bluray-correction.png`, variation 3
from `output/logo-concepts/bluray-marker-2026-11-02/03-correction.png`. It retains
the red handwritten “and Bluray”, curved arrow and insertion caret. The built-in
image generation tool removed the background and tightened the transparent
canvas. The header keeps its existing 192 × 64 display area.

Final extraction prompt: Use case: background-extraction. Edit target: input
image 1, the approved easyDVD and Bluray proofreader logo variation 3. Remove
only the white background, including white letter counters and the disc hole,
to genuine transparency. Preserve exact gold easyDVD lettering, black shadow,
gold ellipse, and ALL red handwritten and Bluray text, red curved arrow and
insertion caret. No redesign, no changes in text, proportions, composition or
texture. Crop canvas tightly around complete artwork with tiny even transparent
margin. Asset type: transparent PNG main app header logo.

Final crop prompt: Precise crop only of this existing transparent logo. Preserve
every pixel of gold easyDVD, disc, black shadow, handwritten red and Bluray,
arrow and caret. Tight crop transparent canvas to the actual artwork bounding
box, removing all excess transparent margin especially below the gold disc and
above the red annotation. Only about 4 pixels empty on each side. Do not redraw
or redesign. Output aspect ratio approximately 2.65:1, the artwork should fill
the frame.

## Disc-format buttons

`dvd-logo.svg` and `bluray-logo.svg` are embedded vector wordmarks for selecting
the target disc format. Sources:

- [DVD logo](https://commons.wikimedia.org/wiki/File:DVD_logo.svg), vector by
  Fleshgrinder / DigitalIceAge based on the DVD Forum mark.
- [Blu-ray Disc logo](https://commons.wikimedia.org/wiki/File:Blu-ray_Disc.svg),
  Blu-ray Disc Association.

Both source pages classify the simple text/shape artwork as public domain;
the format names and logos remain their respective owners' trademarks. The
SVGs have normalized dimensions, namespaces and view boxes for the app's SVG
renderer; their fill follows the app theme and button state.

## Previous header logo

`logo-retro-gold.png` is the approved retro gold variation from the
"Create DVD logo variations" chat, with its white background removed using
the built-in image generation tool.

The PNG is embedded in the app so the portable executable needs no external
logo file. Its transparency supports both light and dark themes.

## Application icon

`app.png` is the gold disc/e monogram on a charcoal tile, derived from the
existing disc icon concept using the built-in image generation tool.
`app.ico` contains PNG frames at 16, 24, 32, 48, 64, 128 and 256 pixels.
The PNG supplies the Fyne window icon; the ICO and release version are embedded
as Windows executable resources during scripted builds.

Prompt: Use case: background-extraction. Remove only the white background outside
the existing rounded-square tile and make it transparent. Preserve the complete
charcoal tile, gold disc/e symbol, proportions, shading and internal dark areas.
Crop to a square with a tiny even transparent margin. No redesign or added text.

## Background extraction prompt

Use case: background-extraction. Input image 1 is the edit target, the approved easyDVD retro gold logo. Remove only the white background, including white inside letter counters and the disc hole, making those areas truly transparent. Preserve the exact existing easyDVD letterforms, wordmark shape, metallic gold gradients, black offset shadow, horizontal disc ellipse, all edges, proportions and spacing. Do not redesign or reinterpret any element. Crop the transparent canvas tightly around the full logo with a tiny even transparent margin, so it is a wide horizontal header asset. No additional text or graphics. Asset type: transparent PNG for an app header.
