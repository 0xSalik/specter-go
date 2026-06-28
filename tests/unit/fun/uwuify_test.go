package fun_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	fun "github.com/0xSalik/specter/internal/commands/fun"
)

func TestUwuifyReplacesR(t *testing.T) {
	out := fun.Uwuify("rrr")
	assert.Equal(t, "www", strings.TrimSpace(strings.TrimSuffix(out, " nya~")))
}

func TestUwuifyReplacesL(t *testing.T) {
	out := fun.Uwuify("lll")
	assert.Equal(t, "www", strings.TrimSpace(strings.TrimSuffix(out, " nya~")))
}

func TestUwuifyChangesSentence(t *testing.T) {
	in := "really lovely world"
	out := fun.Uwuify(in)
	assert.NotEqual(t, in, out)
	assert.Contains(t, out, "w")
}

func TestUwuifyEmpty(t *testing.T) {
	assert.Equal(t, "", fun.Uwuify(""))
}
