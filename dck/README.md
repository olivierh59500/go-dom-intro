# DCK version

This directory contains the construction-kit version of go-dom-intro. The original Go sources are preserved at their original paths (revision `a02358c5470366449f0fadcd25c1ba1f358e44be`), with small asset accessors so both versions use the same embedded resources.

Run the original with `go run ./cmd/domintro` and this version with `go run ./dck/cmd/domintro` from the repository root.

The choreography and assets remain in this repository. Reusable rendering and
effects come from the published `github.com/olivierh59500/democonstructionkit`
module pinned in `go.mod`. Music is opened with `sound.Open`; DCK selects the decoder from the asset and
provides the configured stereo PCM format. The demo keeps its playback level and loop settings.

The eight animated stars use `sprites.AnimatedField` with cached atlas frames.
`presets.DefaultDOMStarOptions` retains their original seeded placement,
independent fractional frame rates and ordered respawn; count, spacing,
frame lifetime and drawing offsets are editable. The original Go source and
assets remain at the repository root.

The four font sizes now use `scrolling.Config.SizeBank` and the editable
`presets.DOMSizeBank` recipe. DCK owns the synchronized offsets, right-edge
font controls, per-size speed and cached repeated text strips. The authored
message stays in `dck/internal/textdata`; a pure test compares the controller
with the previous choreography across 100,000 simulation ticks. Three
moving raster copies color the text through `effects.NewMaskWith`, with editable
phase, blend, viewport and top crop. The scene still chooses its background,
logo, stars and input controls.
