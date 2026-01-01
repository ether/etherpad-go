package io

import (
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/apool"
)

func TestProcessSpaces(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no spaces",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "single space",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "multiple spaces",
			input:    "hello  world",
			expected: "hello&nbsp; world",
		},
		{
			name:     "leading space",
			input:    " hello",
			expected: "&nbsp;hello",
		},
		{
			name:     "trailing space",
			input:    "hello ",
			expected: "hello&nbsp;",
		},
		{
			name:     "spaces with HTML tags",
			input:    "<strong>hello</strong>  world",
			expected: "<strong>hello</strong>&nbsp; world",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := processSpaces(tc.input)
			if result != tc.expected {
				t.Errorf("got %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestFindURLs(t *testing.T) {
	testCases := []struct {
		name          string
		input         string
		expectedCount int
		expectedURLs  []string
	}{
		{
			name:          "no URLs",
			input:         "hello world",
			expectedCount: 0,
			expectedURLs:  nil,
		},
		{
			name:          "single http URL",
			input:         "check out https://example.com for more",
			expectedCount: 1,
			expectedURLs:  []string{"https://example.com"},
		},
		{
			name:          "URL with path",
			input:         "visit https://example.com/path/to/page today",
			expectedCount: 1,
			expectedURLs:  []string{"https://example.com/path/to/page"},
		},
		{
			name:          "multiple URLs",
			input:         "https://one.com and https://two.com",
			expectedCount: 2,
			expectedURLs:  []string{"https://one.com", "https://two.com"},
		},
		{
			name:          "URL with trailing punctuation",
			input:         "Visit https://example.com.",
			expectedCount: 1,
			expectedURLs:  []string{"https://example.com"},
		},
		{
			name:          "www URL",
			input:         "go to www.example.com for info",
			expectedCount: 1,
			expectedURLs:  []string{"www.example.com"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := findURLs(tc.input)
			if len(result) != tc.expectedCount {
				t.Errorf("got %d URLs, want %d", len(result), tc.expectedCount)
				return
			}
			for i, url := range tc.expectedURLs {
				if i < len(result) && result[i].url != url {
					t.Errorf("URL %d: got %q, want %q", i, result[i].url, url)
				}
			}
		})
	}
}

func TestEscapeHTMLContent(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no special chars",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "ampersand",
			input:    "a & b",
			expected: "a &amp; b",
		},
		{
			name:     "less than",
			input:    "a < b",
			expected: "a &lt; b",
		},
		{
			name:     "greater than",
			input:    "a > b",
			expected: "a &gt; b",
		},
		{
			name:     "quotes",
			input:    `say "hello"`,
			expected: "say &#34;hello&#34;",
		},
		{
			name:     "mixed",
			input:    "<script>alert('xss')</script>",
			expected: "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := escapeHTMLContent(tc.input)
			if result != tc.expected {
				t.Errorf("got %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestEscapeHTMLAttribute(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple URL",
			input:    "https://example.com",
			expected: "https://example.com",
		},
		{
			name:     "URL with ampersand",
			input:    "https://example.com?a=1&b=2",
			expected: "https://example.com?a=1&amp;b=2",
		},
		{
			name:     "string with quotes",
			input:    `test"value`,
			expected: "test&#34;value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := escapeHTMLAttribute(tc.input)
			if result != tc.expected {
				t.Errorf("got %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestEncodeWhitespace(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no tabs",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "single tab",
			input:    "hello\tworld",
			expected: "hello    world",
		},
		{
			name:     "multiple tabs",
			input:    "\t\thello",
			expected: "        hello",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := encodeWhitespace(tc.input)
			if result != tc.expected {
				t.Errorf("got %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestContainsInt(t *testing.T) {
	testCases := []struct {
		name     string
		slice    []int
		val      int
		expected bool
	}{
		{
			name:     "empty slice",
			slice:    []int{},
			val:      1,
			expected: false,
		},
		{
			name:     "value present",
			slice:    []int{1, 2, 3},
			val:      2,
			expected: true,
		},
		{
			name:     "value not present",
			slice:    []int{1, 2, 3},
			val:      4,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := containsInt(tc.slice, tc.val)
			if result != tc.expected {
				t.Errorf("got %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestListExists(t *testing.T) {
	lists := []openList{
		{level: 1, listType: "bullet"},
		{level: 2, listType: "number"},
	}

	testCases := []struct {
		name     string
		level    int
		listType string
		expected bool
	}{
		{
			name:     "exists",
			level:    1,
			listType: "bullet",
			expected: true,
		},
		{
			name:     "wrong level",
			level:    3,
			listType: "bullet",
			expected: false,
		},
		{
			name:     "wrong type",
			level:    1,
			listType: "number",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := listExists(lists, tc.level, tc.listType)
			if result != tc.expected {
				t.Errorf("got %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestFilterList(t *testing.T) {
	lists := []openList{
		{level: 1, listType: "bullet"},
		{level: 2, listType: "number"},
		{level: 1, listType: "number"},
	}

	result := filterList(lists, 1, "bullet")

	if len(result) != 2 {
		t.Errorf("got length %d, want 2", len(result))
	}

	for _, l := range result {
		if l.level == 1 && l.listType == "bullet" {
			t.Error("filtered list should not contain {level: 1, listType: bullet}")
		}
	}
}

func TestGetHTMLFromAtext_SimpleText(t *testing.T) {
	exporter := &ExportHtml{}
	pool := apool.NewAPool()

	atext := apool.AText{
		Text:    "Hello World\n",
		Attribs: "|1+c",
	}

	result, err := exporter.getHTMLFromAtext(&pool, atext, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "Hello World") {
		t.Errorf("result should contain 'Hello World', got: %s", result)
	}
	if !strings.Contains(result, "<br>") {
		t.Errorf("result should contain '<br>', got: %s", result)
	}
}

func TestGetHTMLFromAtext_BoldText(t *testing.T) {
	exporter := &ExportHtml{}
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "bold", Value: "true"}, nil)

	atext := apool.AText{
		Text:    "Bold\n",
		Attribs: "*0|1+5",
	}

	result, err := exporter.getHTMLFromAtext(&pool, atext, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "<strong>") {
		t.Errorf("result should contain '<strong>', got: %s", result)
	}
	if !strings.Contains(result, "</strong>") {
		t.Errorf("result should contain '</strong>', got: %s", result)
	}
	if !strings.Contains(result, "Bold") {
		t.Errorf("result should contain 'Bold', got: %s", result)
	}
}

func TestGetHTMLFromAtext_ItalicText(t *testing.T) {
	exporter := &ExportHtml{}
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "italic", Value: "true"}, nil)

	atext := apool.AText{
		Text:    "Italic\n",
		Attribs: "*0|1+7",
	}

	result, err := exporter.getHTMLFromAtext(&pool, atext, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "<em>") {
		t.Errorf("result should contain '<em>', got: %s", result)
	}
	if !strings.Contains(result, "</em>") {
		t.Errorf("result should contain '</em>', got: %s", result)
	}
}

func TestGetHTMLFromAtext_UnderlineText(t *testing.T) {
	exporter := &ExportHtml{}
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "underline", Value: "true"}, nil)

	atext := apool.AText{
		Text:    "Underlined\n",
		Attribs: "*0|1+b",
	}

	result, err := exporter.getHTMLFromAtext(&pool, atext, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "<u>") {
		t.Errorf("result should contain '<u>', got: %s", result)
	}
	if !strings.Contains(result, "</u>") {
		t.Errorf("result should contain '</u>', got: %s", result)
	}
}

func TestGetHTMLFromAtext_StrikethroughText(t *testing.T) {
	exporter := &ExportHtml{}
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "strikethrough", Value: "true"}, nil)

	atext := apool.AText{
		Text:    "Strikethrough\n",
		Attribs: "*0|1+e",
	}

	result, err := exporter.getHTMLFromAtext(&pool, atext, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "<s>") {
		t.Errorf("result should contain '<s>', got: %s", result)
	}
	if !strings.Contains(result, "</s>") {
		t.Errorf("result should contain '</s>', got: %s", result)
	}
}

func TestGetHTMLFromAtext_Heading1(t *testing.T) {
	exporter := &ExportHtml{}
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "heading1", Value: "true"}, nil)

	atext := apool.AText{
		Text:    "Heading\n",
		Attribs: "*0|1+8",
	}

	result, err := exporter.getHTMLFromAtext(&pool, atext, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "<h1>") {
		t.Errorf("result should contain '<h1>', got: %s", result)
	}
	if !strings.Contains(result, "</h1>") {
		t.Errorf("result should contain '</h1>', got: %s", result)
	}
}

func TestGetHTMLFromAtext_Heading2(t *testing.T) {
	exporter := &ExportHtml{}
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "heading2", Value: "true"}, nil)

	atext := apool.AText{
		Text:    "Heading 2\n",
		Attribs: "*0|1+a",
	}

	result, err := exporter.getHTMLFromAtext(&pool, atext, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "<h2>") {
		t.Errorf("result should contain '<h2>', got: %s", result)
	}
	if !strings.Contains(result, "</h2>") {
		t.Errorf("result should contain '</h2>', got: %s", result)
	}
}

func TestGetHTMLFromAtext_MultipleFormats(t *testing.T) {
	exporter := &ExportHtml{}
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "bold", Value: "true"}, nil)
	pool.PutAttrib(apool.Attribute{Key: "italic", Value: "true"}, nil)

	atext := apool.AText{
		Text:    "BoldItalic\n",
		Attribs: "*0*1|1+b",
	}

	result, err := exporter.getHTMLFromAtext(&pool, atext, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "<strong>") {
		t.Errorf("result should contain '<strong>', got: %s", result)
	}
	if !strings.Contains(result, "<em>") {
		t.Errorf("result should contain '<em>', got: %s", result)
	}
}

func TestGetHTMLFromAtext_MixedFormattedText(t *testing.T) {
	exporter := &ExportHtml{}
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "bold", Value: "true"}, nil)

	// "Normal Bold Normal\n"
	atext := apool.AText{
		Text:    "Normal Bold Normal\n",
		Attribs: "+7*0+4+8|1+1",
	}

	result, err := exporter.getHTMLFromAtext(&pool, atext, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "Normal") {
		t.Errorf("result should contain 'Normal', got: %s", result)
	}
	if !strings.Contains(result, "<strong>Bold</strong>") {
		t.Errorf("result should contain '<strong>Bold</strong>', got: %s", result)
	}
}

func TestGetHTMLFromAtext_EmptyText(t *testing.T) {
	exporter := &ExportHtml{}
	pool := apool.NewAPool()

	atext := apool.AText{
		Text:    "\n",
		Attribs: "|1+1",
	}

	result, err := exporter.getHTMLFromAtext(&pool, atext, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "<br>") {
		t.Errorf("result should contain '<br>' for empty line, got: %s", result)
	}
}

func TestGetHTMLFromAtext_MultipleLines(t *testing.T) {
	exporter := &ExportHtml{}
	pool := apool.NewAPool()

	atext := apool.AText{
		Text:    "Line 1\nLine 2\n",
		Attribs: "|1+7|1+7",
	}

	result, err := exporter.getHTMLFromAtext(&pool, atext, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "Line 1") {
		t.Errorf("result should contain 'Line 1', got: %s", result)
	}
	if !strings.Contains(result, "Line 2") {
		t.Errorf("result should contain 'Line 2', got: %s", result)
	}
	if strings.Count(result, "<br>") < 2 {
		t.Errorf("result should contain at least 2 '<br>' tags, got: %s", result)
	}
}

func TestGetHTMLFromAtext_SpecialCharacters(t *testing.T) {
	exporter := &ExportHtml{}
	pool := apool.NewAPool()

	atext := apool.AText{
		Text:    "<script>alert('xss')</script>\n",
		Attribs: "|1+1e",
	}

	result, err := exporter.getHTMLFromAtext(&pool, atext, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be escaped
	if strings.Contains(result, "<script>") {
		t.Errorf("result should have escaped '<script>', got: %s", result)
	}
	if !strings.Contains(result, "&lt;script&gt;") {
		t.Errorf("result should contain '&lt;script&gt;', got: %s", result)
	}
}

func TestGetHTMLFromAtext_WithURL(t *testing.T) {
	exporter := &ExportHtml{}
	pool := apool.NewAPool()

	atext := apool.AText{
		Text:    "Visit https://example.com today\n",
		Attribs: "|1+1g",
	}

	result, err := exporter.getHTMLFromAtext(&pool, atext, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "<a href=") {
		t.Errorf("result should contain '<a href=', got: %s", result)
	}
	if !strings.Contains(result, "https://example.com") {
		t.Errorf("result should contain 'https://example.com', got: %s", result)
	}
	if !strings.Contains(result, "rel=\"noreferrer noopener\"") {
		t.Errorf("result should contain rel attribute, got: %s", result)
	}
}

func TestGetHTMLFromAtext_BulletList(t *testing.T) {
	exporter := &ExportHtml{}
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "list", Value: "bullet1"}, nil)

	atext := apool.AText{
		Text:    "*Item 1\n",
		Attribs: "*0|1+8",
	}

	result, err := exporter.getHTMLFromAtext(&pool, atext, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "<ul") {
		t.Errorf("result should contain '<ul', got: %s", result)
	}
	if !strings.Contains(result, "<li>") {
		t.Errorf("result should contain '<li>', got: %s", result)
	}
	if !strings.Contains(result, "</ul>") {
		t.Errorf("result should contain '</ul>', got: %s", result)
	}
}

func TestGetHTMLFromAtext_NumberedList(t *testing.T) {
	exporter := &ExportHtml{}
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "list", Value: "number1"}, nil)

	atext := apool.AText{
		Text:    "*Item 1\n",
		Attribs: "*0|1+8",
	}

	result, err := exporter.getHTMLFromAtext(&pool, atext, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "<ol") {
		t.Errorf("result should contain '<ol', got: %s", result)
	}
	if !strings.Contains(result, "<li>") {
		t.Errorf("result should contain '<li>', got: %s", result)
	}
	if !strings.Contains(result, "</ol>") {
		t.Errorf("result should contain '</ol>', got: %s", result)
	}
}

func TestGetHTMLFromAtext_WithAuthorColors(t *testing.T) {
	exporter := &ExportHtml{}
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "author", Value: "a.test123"}, nil)

	authorColors := map[string]string{
		"a.test123": "#ff0000",
	}

	atext := apool.AText{
		Text:    "Author text\n",
		Attribs: "*0|1+c",
	}

	result, err := exporter.getHTMLFromAtext(&pool, atext, authorColors)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "<style>") {
		t.Errorf("result should contain '<style>', got: %s", result)
	}
	if !strings.Contains(result, "#ff0000") {
		t.Errorf("result should contain author color '#ff0000', got: %s", result)
	}
	if !strings.Contains(result, "<span class=") {
		t.Errorf("result should contain '<span class=', got: %s", result)
	}
}

func TestGetHTMLFromAtext_UnicodeText(t *testing.T) {
	exporter := &ExportHtml{}
	pool := apool.NewAPool()

	atext := apool.AText{
		Text:    "Hello 世界 🌍\n",
		Attribs: "|1+c",
	}

	result, err := exporter.getHTMLFromAtext(&pool, atext, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "Hello") {
		t.Errorf("result should contain 'Hello', got: %s", result)
	}
	if !strings.Contains(result, "世界") {
		t.Errorf("result should contain '世界', got: %s", result)
	}
	if !strings.Contains(result, "🌍") {
		t.Errorf("result should contain '🌍', got: %s", result)
	}
}

func TestGetHTMLFromAtext_BulletFollowedByNumber(t *testing.T) {
	exporter := &ExportHtml{}
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "list", Value: "bullet1"}, nil)
	pool.PutAttrib(apool.Attribute{Key: "list", Value: "number1"}, nil)

	// Bullet item followed by numbered item
	atext := apool.AText{
		Text:    "*Bullet Item\n*Number Item\n",
		Attribs: "*0|1+d*1|1+d",
	}

	result, err := exporter.getHTMLFromAtext(&pool, atext, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("Result HTML: %s", result)

	// Should have both ul and ol
	if !strings.Contains(result, "<ul") {
		t.Errorf("result should contain '<ul', got: %s", result)
	}
	if !strings.Contains(result, "</ul>") {
		t.Errorf("result should contain '</ul>', got: %s", result)
	}
	if !strings.Contains(result, "<ol") {
		t.Errorf("result should contain '<ol', got: %s", result)
	}
	if !strings.Contains(result, "</ol>") {
		t.Errorf("result should contain '</ol>', got: %s", result)
	}

	// The ul should be closed before ol opens
	ulCloseIndex := strings.Index(result, "</ul>")
	olOpenIndex := strings.Index(result, "<ol")
	if ulCloseIndex > olOpenIndex {
		t.Errorf("</ul> should come before <ol>, got ul close at %d, ol open at %d", ulCloseIndex, olOpenIndex)
	}
}

func TestGetHTMLFromAtext_MultipleBulletItems(t *testing.T) {
	exporter := &ExportHtml{}
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "list", Value: "bullet1"}, nil)

	// Multiple bullet items
	atext := apool.AText{
		Text:    "*Item 1\n*Item 2\n*Item 3\n",
		Attribs: "*0|1+8*0|1+8*0|1+8",
	}

	result, err := exporter.getHTMLFromAtext(&pool, atext, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("Result HTML: %s", result)

	// Should have exactly one ul open and close
	ulOpenCount := strings.Count(result, "<ul")
	ulCloseCount := strings.Count(result, "</ul>")
	if ulOpenCount != 1 {
		t.Errorf("should have exactly 1 <ul, got %d", ulOpenCount)
	}
	if ulCloseCount != 1 {
		t.Errorf("should have exactly 1 </ul>, got %d", ulCloseCount)
	}

	// Should have 3 list items
	liCount := strings.Count(result, "<li>")
	if liCount != 3 {
		t.Errorf("should have 3 <li>, got %d", liCount)
	}
}
