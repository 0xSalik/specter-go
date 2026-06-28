package events_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/0xSalik/specter/internal/events"
	"github.com/0xSalik/specter/internal/modlog"
)

func TestHumanizeBytes(t *testing.T) {
	assert.Equal(t, "512 B", events.HumanizeBytesForTest(512))
	assert.Equal(t, "1.0 KB", events.HumanizeBytesForTest(1024))
	assert.Equal(t, "1.5 KB", events.HumanizeBytesForTest(1536))
	assert.Equal(t, "1.0 MB", events.HumanizeBytesForTest(1024*1024))
}

func TestFormatAttachments(t *testing.T) {
	assert.Empty(t, events.FormatAttachmentsForTest(nil))

	out := events.FormatAttachmentsForTest([]modlog.CachedAttachment{
		{Filename: "cat.png", URL: "https://cdn/cat.png", Size: 2048},
		{Filename: "doc.pdf", ProxyURL: "https://proxy/doc.pdf", Size: 1048576},
	})
	assert.Contains(t, out, "[cat.png](https://cdn/cat.png)")
	assert.Contains(t, out, "2.0 KB")
	assert.Contains(t, out, "[doc.pdf](https://proxy/doc.pdf)")
	assert.Contains(t, out, "1.0 MB")
}

func TestFormatAttachmentsNoURLFallsBackToName(t *testing.T) {
	out := events.FormatAttachmentsForTest([]modlog.CachedAttachment{
		{Filename: "secret.bin", Size: 10},
	})
	assert.Contains(t, out, "secret.bin")
	assert.NotContains(t, out, "](")
}

func TestFormatAttachmentsCapsAtTen(t *testing.T) {
	var atts []modlog.CachedAttachment
	for i := 0; i < 15; i++ {
		atts = append(atts, modlog.CachedAttachment{Filename: "f", Size: 1})
	}
	out := events.FormatAttachmentsForTest(atts)
	assert.Contains(t, out, "and 5 more")
	assert.Equal(t, 10, strings.Count(out, "f (1 B)"))
}
