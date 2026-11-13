package io

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/changeset"
	padModel "github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/utils"
	"golang.org/x/net/html"
)

// An HTML import keeps what etherpad's own HTML export writes: bold,
// italic, underline and strikethrough, bullet, numbered and indented lists
// with their levels, headings (the ep_heading line attribute) and text
// alignment (the ep_align line attribute). etherpad-lite gets the same from
// its content collector and the plugins' collectContent hooks.

type htmlRun struct {
	text  string
	attrs []apool.Attribute
}

type htmlLine struct {
	runs      []htmlRun
	lineAttrs []apool.Attribute
}

type htmlCollector struct {
	lines []*htmlLine
	open  bool
	// afterBlock: a block with content just ended. A <br> right after it is
	// part of that block's line, as in etherpad's own export
	// ("<h1>A</h1><br>"), and does not add an empty line.
	afterBlock bool
}

func (c *htmlCollector) line(block []apool.Attribute) *htmlLine {
	if !c.open {
		c.lines = append(c.lines, &htmlLine{lineAttrs: block})
		c.open = true
	}
	return c.lines[len(c.lines)-1]
}

// endLine closes the open line, if any: the next text starts a new one.
func (c *htmlCollector) endLine() { c.open = false }

// breakLine ends the current line, creating an empty one if none is open
// (<br><br> is an empty line).
func (c *htmlCollector) breakLine(block []apool.Attribute) {
	c.line(block)
	c.open = false
}

var (
	htmlWhitespace = regexp.MustCompile(`[ \t\r\n\f]+`)
	textAlignStyle = regexp.MustCompile(`(?i)text-align\s*:\s*(left|right|center|justify)`)
)

type htmlBlock struct {
	attrs     []apool.Attribute
	listDepth int
	inPre     bool
	// listStart is the start="" of the enclosing <ol>, for its first item.
	listStart *string
}

func withAttr(attrs []apool.Attribute, key, value string) []apool.Attribute {
	out := make([]apool.Attribute, 0, len(attrs)+1)
	for _, a := range attrs {
		if a.Key != key {
			out = append(out, a)
		}
	}
	return append(out, apool.Attribute{Key: key, Value: value})
}

func elementAlign(n *html.Node) string {
	for _, a := range n.Attr {
		if a.Key == "style" {
			if m := textAlignStyle.FindStringSubmatch(a.Val); m != nil {
				return strings.ToLower(m[1])
			}
		}
		if a.Key == "align" {
			return strings.ToLower(a.Val)
		}
	}
	return ""
}

func isFirstItem(n *html.Node) bool {
	for s := n.PrevSibling; s != nil; s = s.PrevSibling {
		if s.Type == html.ElementNode && s.Data == "li" {
			return false
		}
	}
	return true
}

func listKind(n *html.Node) string {
	if n.Data == "ol" {
		return "number"
	}
	for _, a := range n.Attr {
		if a.Key == "class" && strings.Contains(a.Val, "indent") {
			return "indent"
		}
	}
	return "bullet"
}

func (c *htmlCollector) walk(n *html.Node, inline []apool.Attribute, block htmlBlock, kind string) {
	switch n.Type {
	case html.TextNode:
		c.text(n.Data, inline, block)
		return
	case html.ElementNode, html.DocumentNode:
	default:
		return
	}

	isBlock := false
	if n.Type == html.ElementNode {
		switch n.Data {
		case "script", "style", "head", "title", "template":
			return
		case "br":
			if !c.open && c.afterBlock {
				c.afterBlock = false
				return
			}
			c.breakLine(block.attrs)
			c.afterBlock = false
			return
		case "b", "strong":
			inline = withAttr(inline, "bold", "true")
		case "i", "em":
			inline = withAttr(inline, "italic", "true")
		case "u", "ins":
			inline = withAttr(inline, "underline", "true")
		case "s", "strike", "del":
			inline = withAttr(inline, "strikethrough", "true")
		case "h1", "h2", "h3", "h4", "h5", "h6":
			isBlock = true
			block.attrs = withAttr(block.attrs, "heading", n.Data)
		case "ul", "ol":
			isBlock = true
			block.listDepth++
			kind = listKind(n)
			block.listStart = nil
			if n.Data == "ol" {
				for _, a := range n.Attr {
					if a.Key == "start" {
						if v, err := strconv.Atoi(strings.TrimSpace(a.Val)); err == nil && v > 0 {
							start := strconv.Itoa(v)
							block.listStart = &start
						}
					}
				}
			}
		case "li":
			isBlock = true
			depth := block.listDepth
			if depth == 0 {
				depth = 1
			}
			if kind == "" {
				kind = "bullet"
			}
			block.attrs = withAttr(block.attrs, "list", kind+strconv.Itoa(depth))
			if block.listStart != nil && isFirstItem(n) {
				block.attrs = withAttr(block.attrs, "start", *block.listStart)
			}
			block.listStart = nil
		case "pre":
			isBlock = true
			block.inPre = true
		case "p", "div", "blockquote", "section", "article", "header", "footer", "tr", "table", "dt", "dd":
			isBlock = true
		}
		if isBlock {
			if align := elementAlign(n); align != "" && align != "left" {
				block.attrs = withAttr(block.attrs, "align", align)
			}
			c.endLine()
		}
	}
	linesBefore := len(c.lines)
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		c.walk(child, inline, block, kind)
	}
	if isBlock {
		// An empty paragraph is an empty line, as in etherpad-lite (and
		// how word processors write blank lines).
		if len(c.lines) == linesBefore && !c.open && n.Type == html.ElementNode && lineBlocks[n.Data] {
			c.line(block.attrs)
		}
		c.endLine()
		// Etherpad's export writes "<br>" after a heading or paragraph line
		// but not after a list, so a <br> after </ul> is an empty line.
		c.afterBlock = len(c.lines) > linesBefore && n.Data != "ul" && n.Data != "ol"
	}
}

var lineBlocks = map[string]bool{"p": true, "div": true, "li": true, "pre": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true}

func (c *htmlCollector) text(s string, inline []apool.Attribute, block htmlBlock) {
	if block.inPre {
		parts := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
		for i, part := range parts {
			if i > 0 {
				c.breakLine(block.attrs)
			}
			if part != "" {
				l := c.line(block.attrs)
				l.runs = append(l.runs, htmlRun{text: part, attrs: inline})
			}
		}
		return
	}
	s = htmlWhitespace.ReplaceAllString(s, " ")
	// Spaces collapse across elements too ("Need <span> more </span>").
	if !c.open || c.endsWithSpace() {
		s = strings.TrimLeft(s, " ")
	}
	if s == "" {
		return
	}
	l := c.line(block.attrs)
	l.runs = append(l.runs, htmlRun{text: s, attrs: inline})
	c.afterBlock = false
}

func (c *htmlCollector) endsWithSpace() bool {
	if !c.open || len(c.lines) == 0 {
		return false
	}
	runs := c.lines[len(c.lines)-1].runs
	return len(runs) > 0 && strings.HasSuffix(runs[len(runs)-1].text, " ")
}

// parseHTMLLines turns HTML into pad lines.
func parseHTMLLines(src string) ([]*htmlLine, error) {
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		return nil, err
	}
	c := &htmlCollector{}
	c.walk(doc, nil, htmlBlock{}, "")
	// Trailing spaces of a line and empty lines at the end carry nothing.
	for _, l := range c.lines {
		if n := len(l.runs); n > 0 {
			l.runs[n-1].text = strings.TrimRight(l.runs[n-1].text, " ")
		}
	}
	for len(c.lines) > 0 {
		last := c.lines[len(c.lines)-1]
		if len(last.lineAttrs) > 0 || strings.Trim(lineText(last), " ") != "" {
			break
		}
		c.lines = c.lines[:len(c.lines)-1]
	}
	return c.lines, nil
}

func lineText(l *htmlLine) string {
	var sb strings.Builder
	for _, r := range l.runs {
		sb.WriteString(r.text)
	}
	return sb.String()
}

// setPadLines replaces the pad's content with lines in one revision.
func setPadLines(p *padModel.Pad, lines []*htmlLine, authorId string) error {
	if authorId == "" {
		authorId = padModel.SystemAuthorId
	}
	attribs := func(extra ...[]apool.Attribute) changeset.KeepArgs {
		m := changeset.FromString("", &p.Pool)
		m.Set("author", authorId)
		for _, list := range extra {
			for _, a := range list {
				m.Set(a.Key, a.Value)
			}
		}
		return changeset.AttribsString(m.String())
	}

	oldText := p.Text()
	builder := changeset.NewBuilder(utils.RuneCount(oldText))
	for i, l := range lines {
		if len(l.lineAttrs) > 0 {
			marker := []apool.Attribute{{Key: "insertorder", Value: "first"}, {Key: "lmkr", Value: "1"}}
			builder.Insert("*", attribs(marker, l.lineAttrs), nil)
		}
		for _, r := range l.runs {
			// A non-breaking space is a space the author wanted: kept
			// (never collapsed or trimmed above), stored as a space.
			text := strings.ReplaceAll(*padModel.CleanText(r.text), "\u00a0", " ")
			if text != "" {
				builder.Insert(text, attribs(r.attrs), nil)
			}
		}
		if i < len(lines)-1 {
			builder.Insert("\n", attribs(), nil)
		}
	}
	oldBody := strings.TrimSuffix(oldText, "\n")
	if lastNL := strings.LastIndex(oldBody, "\n"); lastNL >= 0 {
		builder.Remove(utils.RuneCount(oldBody[:lastNL+1]), strings.Count(oldBody, "\n"))
		builder.Remove(utils.RuneCount(oldBody[lastNL+1:]), 0)
	} else {
		builder.Remove(utils.RuneCount(oldBody), 0)
	}
	_, err := p.AppendRevision(builder.ToString(), &authorId)
	return err
}
