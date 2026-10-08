// The gonum adapter is a separate module so the meso core stays dependency-free:
// consumers who do not want gonum never pull it into their build graph. It
// requires a published release of the core, never a local replace.
module github.com/andreswebs/meso/gonum

go 1.27.1

require (
	github.com/andreswebs/meso v0.2.0
	gonum.org/v1/gonum v0.17.0
)
