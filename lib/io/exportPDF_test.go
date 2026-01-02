package io

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractEtherpadFromPdf_InvalidFile(t *testing.T) {
	importer := &Importer{}

	content := []byte("not a pdf file")
	_, err := importer.ExtractEtherpadFromPdf(content)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid PDF file")
}

func TestExtractEtherpadFromPdf_NoPdfHeader(t *testing.T) {
	importer := &Importer{}

	content := []byte("ABC")
	_, err := importer.ExtractEtherpadFromPdf(content)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid PDF file")
}

func TestEtherpadExportJSONMarshal(t *testing.T) {
	// Test that EtherpadExport can be marshalled to JSON
	export := &EtherpadExport{
		Pad:       make(map[string]PadData),
		Authors:   make(map[string]GlobalAuthor),
		Revisions: make(map[string]Revision),
		Chats:     make(map[string]ChatMessage),
	}

	export.Pad["pad:test:"] = PadData{
		AText: AText{
			Text:    "Hello World\n",
			Attribs: "*0|1+c",
		},
		Pool: Pool{
			NumToAttrib: map[string][]string{
				"0": {"author", "a.123"},
			},
			NextNum: 1,
		},
		Head:         1,
		ChatHead:     0,
		PublicStatus: false,
	}

	jsonData, err := json.Marshal(export)
	require.NoError(t, err)
	assert.Contains(t, string(jsonData), "Hello World")
	assert.Contains(t, string(jsonData), "pad:test:")
}

func TestExtractTextFromEtherpadJson(t *testing.T) {
	importer := &Importer{}

	// Create a mock Etherpad JSON export
	export := map[string]interface{}{
		"pad:testpad:": map[string]interface{}{
			"atext": map[string]interface{}{
				"text":    "This is the pad content.\nWith multiple lines.\n",
				"attribs": "*0|2+2f",
			},
			"pool": map[string]interface{}{
				"numToAttrib": map[string][]string{
					"0": {"author", "a.123"},
				},
				"nextNum": 1,
			},
		},
	}

	jsonData, err := json.Marshal(export)
	require.NoError(t, err)

	text, err := importer.ExtractTextFromEtherpadJson(jsonData)
	require.NoError(t, err)
	assert.Equal(t, "This is the pad content.\nWith multiple lines.\n", text)
}

func TestExtractTextFromEtherpadJson_InvalidJSON(t *testing.T) {
	importer := &Importer{}

	_, err := importer.ExtractTextFromEtherpadJson([]byte("not json"))
	assert.Error(t, err)
}

func TestExtractTextFromEtherpadJson_NoPadData(t *testing.T) {
	importer := &Importer{}

	jsonData := []byte(`{"globalAuthor:a.123": {"colorId": "#ff0000"}}`)
	_, err := importer.ExtractTextFromEtherpadJson(jsonData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no text content")
}

func TestExtractTextFromEtherpadJson_RealFormat(t *testing.T) {
	importer := &Importer{}

	// This is the actual format from the PDF export
	jsonData := []byte(`{
		"globalAuthor:a.97d6a8f64bef8dec4d30c4634541e29b": {
			"colorId": "#f3a5e7",
			"timestamp": 1767177416,
			"padIDs": "newPad33454",
			"name": "hallo"
		},
		"pad:newPad33454:": {
			"atext": {
				"text": "*Dies ist eine Liste\n\n",
				"attribs": "*0*1*2*3*4+1*0+j|2+2"
			},
			"pool": {
				"numToAttrib": {
					"0": ["author", "a.97d6a8f64bef8dec4d30c4634541e29b"],
					"1": ["insertorder", "first"],
					"2": ["list", "number1"],
					"3": ["lmkr", "1"],
					"4": ["start", "1"]
				},
				"nextNum": 5
			},
			"head": 7,
			"chatHead": -1,
			"publicStatus": false,
			"savedRevisions": []
		}
	}`)

	text, err := importer.ExtractTextFromEtherpadJson(jsonData)
	require.NoError(t, err)
	assert.Equal(t, "*Dies ist eine Liste\n\n", text)
}
