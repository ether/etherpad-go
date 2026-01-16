package ep_align

import (
	"testing"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/hooks/events"
)

// createTestPool creates a test attribute pool with common attributes
func createTestPool() *apool.APool {
	pool := &apool.APool{
		NumToAttrib: make(map[int]apool.Attribute),
		AttribToNum: make(map[apool.Attribute]int),
		NextNum:     0,
	}
	return pool
}

// addAlignAttribute adds an align attribute to the pool and returns the attrib string
func addAlignAttribute(pool *apool.APool, alignValue string) string {
	num := pool.PutAttrib(apool.Attribute{Key: "align", Value: alignValue}, nil)
	// Format: *num*
	return "*" + string(rune('0'+num)) + "*"
}

func TestAnalyzeLine_NoAlignment(t *testing.T) {
	pool := createTestPool()
	aline := ""

	result := analyzeLine(&aline, pool)

	if result != nil {
		t.Errorf("Expected nil alignment, got %v", *result)
	}
}

func TestAnalyzeLine_NilAttribLine(t *testing.T) {
	pool := createTestPool()

	result := analyzeLine(nil, pool)

	if result != nil {
		t.Errorf("Expected nil alignment for nil attrib line, got %v", *result)
	}
}

func TestAnalyzeLine_CenterAlignment(t *testing.T) {
	pool := createTestPool()
	// Add center alignment to pool
	pool.PutAttrib(apool.Attribute{Key: "align", Value: "center"}, nil)

	// Create attrib string: +1*0*1| means 1 char with attrib 0
	aline := "*0|1+1"

	result := analyzeLine(&aline, pool)

	if result == nil {
		t.Fatal("Expected alignment, got nil")
	}
	if *result != "center" {
		t.Errorf("Expected 'center' alignment, got '%s'", *result)
	}
}

func TestAnalyzeLine_LeftAlignment(t *testing.T) {
	pool := createTestPool()
	pool.PutAttrib(apool.Attribute{Key: "align", Value: "left"}, nil)
	aline := "*0|1+1"

	result := analyzeLine(&aline, pool)

	if result == nil {
		t.Fatal("Expected alignment, got nil")
	}
	if *result != "left" {
		t.Errorf("Expected 'left' alignment, got '%s'", *result)
	}
}

func TestAnalyzeLine_RightAlignment(t *testing.T) {
	pool := createTestPool()
	pool.PutAttrib(apool.Attribute{Key: "align", Value: "right"}, nil)
	aline := "*0|1+1"

	result := analyzeLine(&aline, pool)

	if result == nil {
		t.Fatal("Expected alignment, got nil")
	}
	if *result != "right" {
		t.Errorf("Expected 'right' alignment, got '%s'", *result)
	}
}

func TestAnalyzeLine_JustifyAlignment(t *testing.T) {
	pool := createTestPool()
	pool.PutAttrib(apool.Attribute{Key: "align", Value: "justify"}, nil)
	aline := "*0|1+1"

	result := analyzeLine(&aline, pool)

	if result == nil {
		t.Fatal("Expected alignment, got nil")
	}
	if *result != "justify" {
		t.Errorf("Expected 'justify' alignment, got '%s'", *result)
	}
}

func TestGetLineHTMLForExport_NoAlignment(t *testing.T) {
	pool := createTestPool()
	lineContent := "Hello World"
	text := "Hello World"
	aline := ""
	padId := "test-pad"

	event := &events.LineHtmlForExportContext{
		LineContent: &lineContent,
		Apool:       pool,
		AttribLine:  &aline,
		Text:        &text,
		PadId:       &padId,
	}

	GetLineHTMLForExport(event)

	// Should remain unchanged when no alignment
	if *event.LineContent != "Hello World" {
		t.Errorf("Expected unchanged content, got '%s'", *event.LineContent)
	}
}

func TestGetLineHTMLForExport_CenterAlignment(t *testing.T) {
	pool := createTestPool()
	pool.PutAttrib(apool.Attribute{Key: "align", Value: "center"}, nil)

	lineContent := "Hello World"
	text := "*Hello World"
	aline := "*0|1+1"
	padId := "test-pad"

	event := &events.LineHtmlForExportContext{
		LineContent: &lineContent,
		Apool:       pool,
		AttribLine:  &aline,
		Text:        &text,
		PadId:       &padId,
	}

	GetLineHTMLForExport(event)

	expected := "<p style='text-align:center'>Hello World</p>"
	if *event.LineContent != expected {
		t.Errorf("Expected '%s', got '%s'", expected, *event.LineContent)
	}
}

func TestGetLineHTMLForExport_HeadingWithAlignment(t *testing.T) {
	pool := createTestPool()
	pool.PutAttrib(apool.Attribute{Key: "align", Value: "center"}, nil)

	lineContent := "<h1>Hello World</h1>"
	text := "*Hello World"
	aline := "*0|1+1"
	padId := "test-pad"

	event := &events.LineHtmlForExportContext{
		LineContent: &lineContent,
		Apool:       pool,
		AttribLine:  &aline,
		Text:        &text,
		PadId:       &padId,
	}

	GetLineHTMLForExport(event)

	expected := "<h1 style='text-align:center'>Hello World</h1>"
	if *event.LineContent != expected {
		t.Errorf("Expected '%s', got '%s'", expected, *event.LineContent)
	}
}

func TestGetLineHTMLForExport_RemovesLeadingAsterisk(t *testing.T) {
	pool := createTestPool()
	pool.PutAttrib(apool.Attribute{Key: "align", Value: "right"}, nil)

	lineContent := "*Some text"
	text := "*Some text"
	aline := "*0|1+1"
	padId := "test-pad"

	event := &events.LineHtmlForExportContext{
		LineContent: &lineContent,
		Apool:       pool,
		AttribLine:  &aline,
		Text:        &text,
		PadId:       &padId,
	}

	GetLineHTMLForExport(event)

	// Should not contain the asterisk
	expected := "<p style='text-align:right'>Some text</p>"
	if *event.LineContent != expected {
		t.Errorf("Expected '%s', got '%s'", expected, *event.LineContent)
	}
}

func TestGetLinePDFForExport_SetsAlignment(t *testing.T) {
	pool := createTestPool()
	pool.PutAttrib(apool.Attribute{Key: "align", Value: "center"}, nil)

	text := "*Hello World"
	aline := "*0|1+1"
	padId := "test-pad"

	event := &events.LinePDFForExportContext{
		Apool:      pool,
		AttribLine: &aline,
		Text:       &text,
		PadId:      &padId,
		Alignment:  nil,
	}

	GetLinePDFForExport(event)

	if event.Alignment == nil {
		t.Fatal("Expected alignment to be set, got nil")
	}
	if *event.Alignment != "center" {
		t.Errorf("Expected 'center' alignment, got '%s'", *event.Alignment)
	}
}

func TestGetLinePDFForExport_NoAlignment(t *testing.T) {
	pool := createTestPool()

	text := "Hello World"
	aline := ""
	padId := "test-pad"

	event := &events.LinePDFForExportContext{
		Apool:      pool,
		AttribLine: &aline,
		Text:       &text,
		PadId:      &padId,
		Alignment:  nil,
	}

	GetLinePDFForExport(event)

	if event.Alignment != nil {
		t.Errorf("Expected nil alignment, got '%s'", *event.Alignment)
	}
}

func TestGetLineDocxForExport_SetsAlignment(t *testing.T) {
	pool := createTestPool()
	pool.PutAttrib(apool.Attribute{Key: "align", Value: "right"}, nil)

	text := "*Hello World"
	aline := "*0|1+1"
	padId := "test-pad"

	event := &events.LineDocxForExportContext{
		Apool:      pool,
		AttribLine: &aline,
		Text:       &text,
		PadId:      &padId,
		Alignment:  nil,
	}

	GetLineDocxForExport(event)

	if event.Alignment == nil {
		t.Fatal("Expected alignment to be set, got nil")
	}
	if *event.Alignment != "right" {
		t.Errorf("Expected 'right' alignment, got '%s'", *event.Alignment)
	}
}

func TestGetLineDocxForExport_NoAlignment(t *testing.T) {
	pool := createTestPool()

	text := "Hello World"
	aline := ""
	padId := "test-pad"

	event := &events.LineDocxForExportContext{
		Apool:      pool,
		AttribLine: &aline,
		Text:       &text,
		PadId:      &padId,
		Alignment:  nil,
	}

	GetLineDocxForExport(event)

	if event.Alignment != nil {
		t.Errorf("Expected nil alignment, got '%s'", *event.Alignment)
	}
}

func TestGetLineOdtForExport_SetsAlignment(t *testing.T) {
	pool := createTestPool()
	pool.PutAttrib(apool.Attribute{Key: "align", Value: "justify"}, nil)

	text := "*Hello World"
	aline := "*0|1+1"
	padId := "test-pad"

	event := &events.LineOdtForExportContext{
		Apool:      pool,
		AttribLine: &aline,
		Text:       &text,
		PadId:      &padId,
		Alignment:  nil,
	}

	GetLineOdtForExport(event)

	if event.Alignment == nil {
		t.Fatal("Expected alignment to be set, got nil")
	}
	if *event.Alignment != "justify" {
		t.Errorf("Expected 'justify' alignment, got '%s'", *event.Alignment)
	}
}

func TestGetLineOdtForExport_NoAlignment(t *testing.T) {
	pool := createTestPool()

	text := "Hello World"
	aline := ""
	padId := "test-pad"

	event := &events.LineOdtForExportContext{
		Apool:      pool,
		AttribLine: &aline,
		Text:       &text,
		PadId:      &padId,
		Alignment:  nil,
	}

	GetLineOdtForExport(event)

	if event.Alignment != nil {
		t.Errorf("Expected nil alignment, got '%s'", *event.Alignment)
	}
}

func TestGetLineHTMLForExport_AllAlignmentTypes(t *testing.T) {
	alignments := []string{"left", "center", "right", "justify"}

	for _, alignment := range alignments {
		t.Run(alignment, func(t *testing.T) {
			pool := createTestPool()
			pool.PutAttrib(apool.Attribute{Key: "align", Value: alignment}, nil)

			lineContent := "Test content"
			text := "*Test content"
			aline := "*0|1+1"
			padId := "test-pad"

			event := &events.LineHtmlForExportContext{
				LineContent: &lineContent,
				Apool:       pool,
				AttribLine:  &aline,
				Text:        &text,
				PadId:       &padId,
			}

			GetLineHTMLForExport(event)

			expected := "<p style='text-align:" + alignment + "'>Test content</p>"
			if *event.LineContent != expected {
				t.Errorf("Expected '%s', got '%s'", expected, *event.LineContent)
			}
		})
	}
}
