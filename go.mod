module github.com/kfet/pinoauth

go 1.21

// v0.2.0 and v0.2.1 were tagged then re-tagged at different commits
// while the public release was being settled. The Go module proxy
// caches the original (pre-fix) content under these versions; the
// git tags point at the corrected commits. Use v0.2.2 or later.
retract (
	v0.2.0
	v0.2.1
)
