package pad

import (
	"github.com/ether/etherpad-go/lib/models/pad"
	"testing"
)

func TestCleanText(t *testing.T) {
	testCases := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"\n", "\n"},
		{"x", "x"},
		{"x\n", "x\n"},
		{"x\ny\n", "x\ny\n"},
		{"x\ry\n", "x\ny\n"},
		{"x\r\ny\n", "x\ny\n"},
		{"x\r\r\ny\n", "x\n\ny\n"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			got := pad.CleanText(tc.input)
			if *got != tc.want {
				t.Errorf("CleanText(%q) = %q; want %q", tc.input, *got, tc.want)
			}
		})
	}
}
