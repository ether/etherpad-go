package io

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHtmlToText_SimpleText(t *testing.T) {
	importer := &Importer{}

	html := "<html><body><p>Hello World</p></body></html>"
	result, err := importer.htmlToText(html)

	require.NoError(t, err)
	assert.Contains(t, result, "Hello World")
	assert.True(t, strings.HasSuffix(result, "\n"))
}

func TestHtmlToText_MultipleElements(t *testing.T) {
	importer := &Importer{}

	html := "<html><body><p>Line 1</p><p>Line 2</p></body></html>"
	result, err := importer.htmlToText(html)

	require.NoError(t, err)
	assert.Contains(t, result, "Line 1")
	assert.Contains(t, result, "Line 2")
}

func TestHtmlToText_WithBreak(t *testing.T) {
	importer := &Importer{}

	html := "<html><body>Line 1<br>Line 2</body></html>"
	result, err := importer.htmlToText(html)

	require.NoError(t, err)
	lines := strings.Split(result, "\n")
	assert.GreaterOrEqual(t, len(lines), 2)
}

func TestHtmlToText_WithHeadings(t *testing.T) {
	importer := &Importer{}

	html := "<html><body><h1>Title</h1><p>Content</p></body></html>"
	result, err := importer.htmlToText(html)

	require.NoError(t, err)
	assert.Contains(t, result, "Title")
	assert.Contains(t, result, "Content")
}

func TestHtmlToText_SkipsScriptAndStyle(t *testing.T) {
	importer := &Importer{}

	html := "<html><head><style>body{color:red}</style></head><body><script>alert('hi')</script><p>Visible</p></body></html>"
	result, err := importer.htmlToText(html)

	require.NoError(t, err)
	assert.Contains(t, result, "Visible")
	assert.NotContains(t, result, "alert")
	assert.NotContains(t, result, "color:red")
}

func TestHtmlToText_ListItems(t *testing.T) {
	importer := &Importer{}

	html := "<html><body><ul><li>Item 1</li><li>Item 2</li></ul></body></html>"
	result, err := importer.htmlToText(html)

	require.NoError(t, err)
	assert.Contains(t, result, "Item 1")
	assert.Contains(t, result, "Item 2")
}

func TestHtmlToText_Table(t *testing.T) {
	importer := &Importer{}

	html := "<html><body><table><tr><td>Cell 1</td><td>Cell 2</td></tr></table></body></html>"
	result, err := importer.htmlToText(html)

	require.NoError(t, err)
	assert.Contains(t, result, "Cell 1")
	assert.Contains(t, result, "Cell 2")
}

func TestHtmlToText_EmptyHtml(t *testing.T) {
	importer := &Importer{}

	html := "<html><body></body></html>"
	result, err := importer.htmlToText(html)

	require.NoError(t, err)
	// Should at least end with newline
	assert.True(t, strings.HasSuffix(result, "\n"))
}

func TestHtmlToText_PlainText(t *testing.T) {
	importer := &Importer{}

	// Just text without proper HTML tags
	html := "Just plain text"
	result, err := importer.htmlToText(html)

	require.NoError(t, err)
	assert.Contains(t, result, "Just plain text")
}

func TestHtmlToText_MultipleDivs(t *testing.T) {
	importer := &Importer{}

	html := "<html><body><div>Div 1</div><div>Div 2</div><div>Div 3</div></body></html>"
	result, err := importer.htmlToText(html)

	require.NoError(t, err)
	assert.Contains(t, result, "Div 1")
	assert.Contains(t, result, "Div 2")
	assert.Contains(t, result, "Div 3")
}

func TestHtmlToText_NestedElements(t *testing.T) {
	importer := &Importer{}

	html := "<html><body><div><p><strong>Bold</strong> and <em>italic</em></p></div></body></html>"
	result, err := importer.htmlToText(html)

	require.NoError(t, err)
	assert.Contains(t, result, "Bold")
	assert.Contains(t, result, "and")
	assert.Contains(t, result, "italic")
}

func TestParseHTMLWithFormatting_Bold(t *testing.T) {
	importer := &Importer{}

	html := "<html><body><strong>Bold text</strong></body></html>"
	result, err := importer.parseHTMLWithFormatting(html)

	require.NoError(t, err)
	assert.Contains(t, result.text, "Bold text")
	assert.Greater(t, len(result.segments), 0, "Should have formatting segments")

	// Check that bold attribute is set
	found := false
	for _, seg := range result.segments {
		if seg.attributes["bold"] == "true" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should have bold attribute")
}

func TestParseHTMLWithFormatting_Italic(t *testing.T) {
	importer := &Importer{}

	html := "<html><body><em>Italic text</em></body></html>"
	result, err := importer.parseHTMLWithFormatting(html)

	require.NoError(t, err)
	assert.Contains(t, result.text, "Italic text")

	found := false
	for _, seg := range result.segments {
		if seg.attributes["italic"] == "true" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should have italic attribute")
}

func TestParseHTMLWithFormatting_Underline(t *testing.T) {
	importer := &Importer{}

	html := "<html><body><u>Underlined text</u></body></html>"
	result, err := importer.parseHTMLWithFormatting(html)

	require.NoError(t, err)
	assert.Contains(t, result.text, "Underlined text")

	found := false
	for _, seg := range result.segments {
		if seg.attributes["underline"] == "true" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should have underline attribute")
}

func TestParseHTMLWithFormatting_Strikethrough(t *testing.T) {
	importer := &Importer{}

	html := "<html><body><s>Strikethrough text</s></body></html>"
	result, err := importer.parseHTMLWithFormatting(html)

	require.NoError(t, err)
	assert.Contains(t, result.text, "Strikethrough text")

	found := false
	for _, seg := range result.segments {
		if seg.attributes["strikethrough"] == "true" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should have strikethrough attribute")
}

func TestParseHTMLWithFormatting_NestedFormatting(t *testing.T) {
	importer := &Importer{}

	html := "<html><body><u><s>Underline and strikethrough</s></u></body></html>"
	result, err := importer.parseHTMLWithFormatting(html)

	require.NoError(t, err)
	assert.Contains(t, result.text, "Underline and strikethrough")

	// Check for segment with both attributes
	foundBoth := false
	for _, seg := range result.segments {
		if seg.attributes["underline"] == "true" && seg.attributes["strikethrough"] == "true" {
			foundBoth = true
			break
		}
	}
	assert.True(t, foundBoth, "Should have segment with both underline and strikethrough")
}

func TestParseHTMLWithFormatting_BulletList(t *testing.T) {
	importer := &Importer{}

	html := `<html><body><ul class="bullet"><li>Item 1</li><li>Item 2</li></ul></body></html>`
	result, err := importer.parseHTMLWithFormatting(html)

	require.NoError(t, err)
	assert.Contains(t, result.text, "Item 1")
	assert.Contains(t, result.text, "Item 2")

	// Should have list attribute
	foundList := false
	for _, seg := range result.segments {
		if strings.HasPrefix(seg.attributes["list"], "bullet") {
			foundList = true
			break
		}
	}
	assert.True(t, foundList, "Should have list attribute for bullet list")
}

func TestParseHTMLWithFormatting_NumberedList(t *testing.T) {
	importer := &Importer{}

	html := `<html><body><ol class="number"><li>First</li><li>Second</li></ol></body></html>`
	result, err := importer.parseHTMLWithFormatting(html)

	require.NoError(t, err)
	assert.Contains(t, result.text, "First")
	assert.Contains(t, result.text, "Second")

	// Should have number list attribute
	foundList := false
	for _, seg := range result.segments {
		if strings.HasPrefix(seg.attributes["list"], "number") {
			foundList = true
			break
		}
	}
	assert.True(t, foundList, "Should have list attribute for numbered list")
}

func TestParseHTMLWithFormatting_MixedContent(t *testing.T) {
	importer := &Importer{}

	html := `<html><body>Normal <strong>bold</strong> and <em>italic</em> text</body></html>`
	result, err := importer.parseHTMLWithFormatting(html)

	require.NoError(t, err)
	assert.Contains(t, result.text, "Normal")
	assert.Contains(t, result.text, "bold")
	assert.Contains(t, result.text, "italic")

	// Check segments
	var boldSegment, italicSegment *htmlSegment
	for i := range result.segments {
		if result.segments[i].attributes["bold"] == "true" {
			boldSegment = &result.segments[i]
		}
		if result.segments[i].attributes["italic"] == "true" {
			italicSegment = &result.segments[i]
		}
	}

	assert.NotNil(t, boldSegment, "Should have bold segment")
	assert.NotNil(t, italicSegment, "Should have italic segment")
}

func TestParseHTMLWithFormatting_Heading(t *testing.T) {
	importer := &Importer{}

	html := "<html><body><h1>Title</h1><p>Content</p></body></html>"
	result, err := importer.parseHTMLWithFormatting(html)

	require.NoError(t, err)
	assert.Contains(t, result.text, "Title")
	assert.Contains(t, result.text, "Content")

	// Should have heading attribute
	foundHeading := false
	for _, seg := range result.segments {
		if seg.attributes["heading"] == "h1" {
			foundHeading = true
			break
		}
	}
	assert.True(t, foundHeading, "Should have heading attribute")
}

func TestSetPadRaw_InvalidJSON(t *testing.T) {
	importer := &Importer{}

	content := []byte("not valid json")
	err := importer.SetPadRaw("testpad", content, "author1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid etherpad JSON")
}

func TestSetPadRaw_NoPadData(t *testing.T) {
	importer := &Importer{}

	content := []byte(`{"someKey": "someValue"}`)
	err := importer.SetPadRaw("testpad", content, "author1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no pad data found")
}

func TestExtractTextFromDocx_InvalidFile(t *testing.T) {
	importer := &Importer{}

	content := []byte("not a zip file")
	_, err := importer.ExtractTextFromDocx(content)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid DOCX file")
}

func TestExtractTextFromOdt_InvalidFile(t *testing.T) {
	importer := &Importer{}

	content := []byte("not a zip file")
	_, err := importer.ExtractTextFromOdt(content)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ODT file")
}

func TestExtractTextFromRtf_InvalidFile(t *testing.T) {
	importer := &Importer{}

	content := []byte("not an rtf file")
	_, err := importer.ExtractTextFromRtf(content)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid RTF file")
}

func TestExtractTextFromRtf_SimpleRtf(t *testing.T) {
	importer := &Importer{}

	// Simple RTF content
	content := []byte(`{\rtf1\ansi Hello World}`)
	result, err := importer.ExtractTextFromRtf(content)

	require.NoError(t, err)
	assert.Contains(t, result, "Hello World")
}

func TestExtractTextFromRtf_WithParagraphs(t *testing.T) {
	importer := &Importer{}

	// RTF with paragraph breaks
	content := []byte(`{\rtf1\ansi Line 1\par Line 2\par Line 3}`)
	result, err := importer.ExtractTextFromRtf(content)

	require.NoError(t, err)
	assert.Contains(t, result, "Line 1")
	assert.Contains(t, result, "Line 2")
	assert.Contains(t, result, "Line 3")
	// Should have newlines
	lines := strings.Split(result, "\n")
	assert.GreaterOrEqual(t, len(lines), 3)
}

func TestExtractTextFromPdf_InvalidFile(t *testing.T) {
	importer := &Importer{}

	content := []byte("not a pdf file")
	_, err := importer.ExtractTextFromPdf(content)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid PDF file")
}

func TestExtractTextFromDocxXML_SimpleText(t *testing.T) {
	importer := &Importer{}

	// Simple DOCX XML structure
	xmlContent := []byte(`<?xml version="1.0"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Hello World</w:t></w:r></w:p>
  </w:body>
</w:document>`)

	result, err := importer.extractTextFromDocxXML(xmlContent)

	require.NoError(t, err)
	assert.Contains(t, result, "Hello World")
}

func TestExtractTextFromDocxXML_MultipleParagraphs(t *testing.T) {
	importer := &Importer{}

	xmlContent := []byte(`<?xml version="1.0"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Paragraph 1</w:t></w:r></w:p>
    <w:p><w:r><w:t>Paragraph 2</w:t></w:r></w:p>
  </w:body>
</w:document>`)

	result, err := importer.extractTextFromDocxXML(xmlContent)

	require.NoError(t, err)
	assert.Contains(t, result, "Paragraph 1")
	assert.Contains(t, result, "Paragraph 2")
}

func TestExtractTextFromOdtXML_SimpleText(t *testing.T) {
	importer := &Importer{}

	// Simple ODT XML structure
	xmlContent := []byte(`<?xml version="1.0"?>
<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
                         xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0">
  <office:body>
    <office:text>
      <text:p>Hello World</text:p>
    </office:text>
  </office:body>
</office:document-content>`)

	result, err := importer.extractTextFromOdtXML(xmlContent)

	require.NoError(t, err)
	assert.Contains(t, result, "Hello World")
}

func TestExtractTextFromOdtXML_MultipleParagraphs(t *testing.T) {
	importer := &Importer{}

	xmlContent := []byte(`<?xml version="1.0"?>
<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
                         xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0">
  <office:body>
    <office:text>
      <text:p>Paragraph 1</text:p>
      <text:p>Paragraph 2</text:p>
    </office:text>
  </office:body>
</office:document-content>`)

	result, err := importer.extractTextFromOdtXML(xmlContent)

	require.NoError(t, err)
	assert.Contains(t, result, "Paragraph 1")
	assert.Contains(t, result, "Paragraph 2")
}

func TestExtractTextFromOdtXML_WithHeading(t *testing.T) {
	importer := &Importer{}

	xmlContent := []byte(`<?xml version="1.0"?>
<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
                         xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0">
  <office:body>
    <office:text>
      <text:h>Title</text:h>
      <text:p>Content</text:p>
    </office:text>
  </office:body>
</office:document-content>`)

	result, err := importer.extractTextFromOdtXML(xmlContent)

	require.NoError(t, err)
	assert.Contains(t, result, "Title")
	assert.Contains(t, result, "Content")
}
