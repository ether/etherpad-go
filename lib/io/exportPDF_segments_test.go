package io

import (
	"testing"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLineSegmentsPluginAttributes(t *testing.T) {
	pool := apool.NewAPool()
	mono := pool.PutAttrib(apool.Attribute{Key: "font-family", Value: "courier"}, nil)
	size := pool.PutAttrib(apool.Attribute{Key: "font-size", Value: "24px"}, nil)
	red := pool.PutAttrib(apool.Attribute{Key: "color", Value: "red"}, nil)
	require.Equal(t, []int{0, 1, 2}, []int{mono, size, red})

	e := &ExportPDF{}
	// "code" monospace, "big" 24px and red, " plain" without attributes.
	segs, _, err := e.parseLineSegments("codebig plain", "*0+4*1*2+3+6", &pool, nil)
	require.NoError(t, err)
	require.Len(t, segs, 3)

	assert.Equal(t, "code", segs[0].text)
	assert.True(t, segs[0].mono)

	assert.Equal(t, "big", segs[1].text)
	assert.InDelta(t, 18.0, segs[1].size, 0.01) // 24px = 18pt
	require.NotNil(t, segs[1].color)
	assert.Equal(t, fontColors["red"], *segs[1].color)

	assert.Equal(t, " plain", segs[2].text)
	assert.False(t, segs[2].mono)
	assert.Zero(t, segs[2].size)
	assert.Nil(t, segs[2].color)
}

func TestParseLineSegmentsIgnoresBadSizes(t *testing.T) {
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "font-size", Value: "100000px"}, nil)
	pool.PutAttrib(apool.Attribute{Key: "font-size", Value: "huge"}, nil)

	e := &ExportPDF{}
	segs, _, err := e.parseLineSegments("ab", "*0+1*1+1", &pool, nil)
	require.NoError(t, err)
	for _, s := range segs {
		assert.Zero(t, s.size, s.text)
	}
}

func TestSplitLinks(t *testing.T) {
	segs := splitLinks([]textSegment{{text: "see https://example.com/x and www.etherpad.org."}})
	var links []string
	var text string
	for _, s := range segs {
		text += s.text
		if s.link != "" {
			links = append(links, s.link)
		}
	}
	assert.Equal(t, "see https://example.com/x and www.etherpad.org.", text)
	assert.Equal(t, []string{"https://example.com/x", "http://www.etherpad.org"}, links)
}
