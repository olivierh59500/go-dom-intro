# DCK version

This directory contains the construction-kit version of go-dom-intro. The original Go sources are preserved at their original paths (revision `a02358c5470366449f0fadcd25c1ba1f358e44be`), with small asset accessors so both versions use the same embedded resources.

Run the original with `go run ./cmd/domintro` and this version with `go run ./dck/cmd/domintro` from the repository root.

The choreography and assets remain in this repository. Reusable rendering and
effects come from the published `github.com/olivierh59500/democonstructionkit`
module pinned in `go.mod`. Go downloads the dependencies automatically, including
`github.com/olivierh59500/ym-player v1.0.0` for YM playback. Second Reality retains its original ST3 music synchronization.
