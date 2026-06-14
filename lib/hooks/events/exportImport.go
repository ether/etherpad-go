package events

import "strings"

// ExportFileNameContext is passed to exportFileName hooks before the export's
// download filename is set. First non-empty SetFileName wins. The extension is
// chosen by core and cannot be overridden (security).
type ExportFileNameContext struct {
	PadId      string
	ReadOnlyId string
	ExportType string

	fileName string
}

func (c *ExportFileNameContext) SetFileName(name string) {
	if c.fileName == "" {
		c.fileName = name
	}
}
func (c *ExportFileNameContext) FileName() string { return c.fileName }

// StylesForExportContext is passed to stylesForExport hooks during HTML export.
// Each AddStyle appends CSS; all are concatenated into the document <style>.
type StylesForExportContext struct {
	PadId string

	styles strings.Builder
}

func (c *StylesForExportContext) AddStyle(css string) { c.styles.WriteString(css) }
func (c *StylesForExportContext) Styles() string      { return c.styles.String() }

// ExportHTMLAdditionalContentContext is passed to exportHTMLAdditionalContent
// hooks during HTML export. Each Add appends HTML to the exported body.
type ExportHTMLAdditionalContentContext struct {
	PadId string

	content strings.Builder
}

func (c *ExportHTMLAdditionalContentContext) Add(html string) { c.content.WriteString(html) }
func (c *ExportHTMLAdditionalContentContext) Content() string { return c.content.String() }

// ExportHTMLSendContext is passed to exportHTMLSend hooks just before the HTML
// export response is sent. A callback may replace the document via *ctx.HTML;
// the caller reads HTML back after the hooks run.
type ExportHTMLSendContext struct {
	PadId string
	HTML  *string
}

// ImportContext is passed to import hooks before the built-in file-extension
// dispatch. A plugin handling a (custom) format either fully handles it and calls
// Handle(), or hands back converted content via SetHTML/SetText (which also marks
// it handled) for core to import. If no callback handles it, the built-in
// importer runs. Content is the raw uploaded bytes.
type ImportContext struct {
	FileEnding string
	PadId      string
	AuthorId   string
	Content    []byte

	handled bool
	html    *string
	text    *string
}

func (c *ImportContext) Handle() { c.handled = true }
func (c *ImportContext) SetHTML(html string) {
	c.handled = true
	c.html = &html
}
func (c *ImportContext) SetText(text string) {
	c.handled = true
	c.text = &text
}
func (c *ImportContext) Handled() bool { return c.handled }
func (c *ImportContext) HTML() (string, bool) {
	if c.html == nil {
		return "", false
	}
	return *c.html, true
}
func (c *ImportContext) Text() (string, bool) {
	if c.text == nil {
		return "", false
	}
	return *c.text, true
}

// ImportEtherpadContext is passed to importEtherpad hooks after a .etherpad file
// is parsed, before its records are persisted. Data is the parsed top-level JSON
// object; plugins may inspect or augment it. (lite's prefix-based extra-record /
// temporary pad.db model is intentionally not ported.)
type ImportEtherpadContext struct {
	PadId    string
	SrcPadId string
	Data     map[string]any
}
