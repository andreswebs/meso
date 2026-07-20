// The gonum adapter is a separate module so the meso core stays dependency-free:
// consumers who do not want gonum never pull it into their build graph. The
// gonum dependency lives here alongside a local replace of the core, which is
// not yet published under a release tag.
module github.com/andreswebs/meso/gonum

go 1.26.5

require (
	github.com/andreswebs/meso v0.0.0
	gonum.org/v1/gonum v0.17.0
)

replace github.com/andreswebs/meso => ../
