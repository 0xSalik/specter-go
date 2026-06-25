package embed_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/salik/specter/internal/embed"
)

func TestAsErrorColor(t *testing.T) {
	e := embed.New(nil, "").Title("x").AsError().Build()
	assert.Equal(t, embed.ErrorColor, e.Color)
	assert.Equal(t, 0xED4245, e.Color)
}

func TestAsSuccessColor(t *testing.T) {
	e := embed.New(nil, "").Title("x").AsSuccess().Build()
	assert.Equal(t, embed.SuccessColor, e.Color)
	assert.Equal(t, 0x57F287, e.Color)
}

func TestFieldsAppend(t *testing.T) {
	e := embed.New(nil, "").
		Field("a", "1", true).
		Field("b", "2", false).Build()
	require.Len(t, e.Fields, 2)
	assert.Equal(t, "a", e.Fields[0].Name)
	assert.Equal(t, "2", e.Fields[1].Value)
	assert.True(t, e.Fields[0].Inline)
	assert.False(t, e.Fields[1].Inline)
}

func TestBuildTitleDescription(t *testing.T) {
	e := embed.New(nil, "").Title("Title").Description("Body").Build()
	require.NotNil(t, e)
	assert.Equal(t, "Title", e.Title)
	assert.Equal(t, "Body", e.Description)
}

func TestDefaultGuildColorWhenNoStore(t *testing.T) {
	// With no DB initialized, an info embed falls back to the default color.
	e := embed.New(nil, "some-guild").Build()
	assert.Equal(t, embed.DefaultColor, e.Color)
}

func TestParseHexColor(t *testing.T) {
	assert.Equal(t, 0x5865F2, embed.ParseHexColor("#5865F2"))
	assert.Equal(t, 0x000000, embed.ParseHexColor("#000000"))
	assert.Equal(t, embed.DefaultColor, embed.ParseHexColor("not-a-color"))
}

func TestValidHexColor(t *testing.T) {
	assert.True(t, embed.ValidHexColor("#FFFFFF"))
	assert.True(t, embed.ValidHexColor("123abc"))
	assert.False(t, embed.ValidHexColor("#FFF"))
	assert.False(t, embed.ValidHexColor("#GGGGGG"))
}
