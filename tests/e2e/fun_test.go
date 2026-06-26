//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	fun "github.com/salik/specter/internal/commands/fun"
)

// TestUwuifyDeterministic exercises the pure transformation end-to-end with the
// same code path the /uwuify command uses.
func TestUwuifyDeterministic(t *testing.T) {
	out := fun.Uwuify("really lovely")
	assert.Contains(t, out, "w")
	assert.False(t, strings.Contains(out, "really"))
}
