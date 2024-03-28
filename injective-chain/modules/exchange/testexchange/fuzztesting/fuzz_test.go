package fuzztesting

import (
	"testing"
)

func FuzzTest(f *testing.F) {
	f.Add(3, 0, 0, 0, 1, 10, []byte{})

	f.Fuzz(fuzzTestAsGinkgo)
}
