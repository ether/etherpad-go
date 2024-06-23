package changeset

import (
	"github.com/ether/etherpad-go/lib/apool"
	"slices"
	"strconv"
	"testing"
)

func TestRejectsInvalidAttributeString(t *testing.T) {
	var testcases = []string{"x", "*0+1", "*A", "*0$" + "*", "0", "*-1"}
	for _, testcase := range testcases {
		_, err := DecodeAttribString(testcase)

		if err == nil {
			t.Error("Expected error, got nil")
		}
	}
}

func TestAcceptsValidAttributeString(t *testing.T) {
	n := 37
	keys := make([]int, n)
	for i := range keys {
		keys[i] = i
	}
	var mappings = map[string][]int{
		"":    {},
		"*0":  {0},
		"*a":  {10},
		"*z":  {35},
		"*10": {36},
		"*0*1*2*3*4*5*6*7*8*9*a*b*c*d*e*f*g*h*i*j*k*l*m*n*o*p*q*r*s*t*u*v*w*x*y*z*10": keys,
	}

	for key, value := range mappings {
		attribs, err := DecodeAttribString(key)
		if err != nil {
			t.Error("Expected nil, got ", err)
		}
		if !slices.Equal(attribs, value) {
			t.Error("Expected ", value, ", got ", attribs)
		}
	}
}

func TestEncodeAttribString(t *testing.T) {
	var res, err = encodeAttribString([]int{0, 1})

	if err != nil {
		t.Error("Expected nil, got ", err)
	}

	if *res != "*0*1" {
		t.Error("Expected *0*1, got ", res)
	}
}

func TestEncodeRejectsInvalidInput(t *testing.T) {
	var testCases = [][]int{
		{-1},
	}

	for _, testCase := range testCases {
		_, err := encodeAttribString(testCase)
		if err == nil {
			t.Error("Expected error, got nil")
		}
	}
}

func TestAcceptsValidAttributeStringInEncode(t *testing.T) {
	n := 37
	keys := make([]int, n)
	for i := range keys {
		keys[i] = i
	}
	var mappings = map[string][]int{
		"":    {},
		"*0":  {0},
		"*a":  {10},
		"*z":  {35},
		"*10": {36},
		"*0*1*2*3*4*5*6*7*8*9*a*b*c*d*e*f*g*h*i*j*k*l*m*n*o*p*q*r*s*t*u*v*w*x*y*z*10": keys,
	}

	for key, value := range mappings {
		attribs, err := encodeAttribString(value)
		if err != nil {
			t.Error("Expected nil, got ", err)
		}
		if *attribs != key {
			t.Error("Expected ", value, ", got ", attribs)
		}
	}
}

func TestRejectsInvalidAttribsFromNums(t *testing.T) {
	var pool, _ = PrepareAttribPool(t)
	var testcases = []int{
		-1,
		9999,
	}

	for _, testcase := range testcases {
		_, err := attribsFromNums([]int{testcase}, pool)
		if err == nil {
			t.Error("Expected error, got nil " + strconv.Itoa(testcase))
		}
	}
}

func TestAcceptsValidInputs(t *testing.T) {
	var pool, attribs = PrepareAttribPool(t)
	var testCases = [][]int{
		{0},
		{1},
		{0, 1},
		{1, 0},
	}

	var testCases2 = [][][]string{
		{attribs[0]},
		{attribs[1]},
		{attribs[0], attribs[1]},
		{attribs[1], attribs[0]},
	}

	for i, testCase := range testCases {
		attrib, err := attribsFromNums(testCase, pool)
		if err != nil {
			t.Error("Expected nil, got ", err)
		}

		for j, attrib := range *attrib {
			var convertedAttrib, _ = StringToAttrib(testCases2[i][j])
			if attrib != *convertedAttrib {
				t.Error("Expected ", testCases2[i][j], ", got ", attrib)
			}
		}
	}
}

func TestReuseExistingPoolEntries(t *testing.T) {
	var pool, attribs = PrepareAttribPool(t)

	var testCases = [][]int{
		{0},
		{1},
		{0, 1},
		{1, 0},
	}

	var testCases2 = [][][]string{
		{attribs[0]},
		{attribs[1]},
		{attribs[0], attribs[1]},
		{attribs[1], attribs[0]},
	}

	for i, testCase := range testCases {
		attribRetrieved, err := attribsFromNums(testCase, pool)

		if err != nil {
			t.Error("Expected nil, got ", err)
		}

		for j, attrib := range *attribRetrieved {
			var convertedAttrib, _ = StringToAttrib(testCases2[i][j])
			if attrib != *convertedAttrib {
				t.Error("Expected ", testCases2[i][j], ", got ", attrib)
			}
		}

		if getPoolSize(t) != len(attribs) {
			t.Error("Expected ", len(attribs)-1, ", got ", getPoolSize(t))
		}
	}
}

func TestInsertNewAttributesIntoPool(t *testing.T) {
	var pool, attribs = PrepareAttribPool(t)
	var testCases = [][][]string{
		{{"k", "v"}},
		{attribs[0], {"k", "v"}},
		{{"k", "v"}, attribs[0]},
	}

	var testCases2 = [][]int{
		{len(attribs)},
		{0}, {len(attribs)},
		{len(attribs), 0},
	}

	for i, testCase := range testCases {
		var attrArr = make([]apool.Attribute, len(testCase))
		for j, attrib := range testCase {
			retrievedAttr, err := StringToAttrib(attrib)
			if err != nil {
				t.Error("Expected nil, got ", err)
			}
			attrArr[j] = *retrievedAttr
		}

		var got = attribsToNums(attrArr, &pool)
		if !slices.Equal(got, testCases2[i]) {
			t.Error("Expected ", testCases2[i], ", got ", got)
		}
	}
}
