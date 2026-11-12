package changeset

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The cases of etherpad-lite's easysync-assembler spec that
// SmartOpAssembler_test.go did not have yet.

func TestSmartOpAssemblerParity(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"ignores additional pure keeps (no attributes)", "-c*3*4+6|1+1=5", "-c*3*4+6|1+1"},
		{"merges consecutive + ops without multiline", "-c*3*4+6*3*4+1*3*4+9=5", "-c*3*4+g"},
		{"merges consecutive + ops with multiline", "-c*3*4+6*3*4|1+1*3*4|9+f*3*4+k=5", "-c*3*4|a+m*3*4+k"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ops, err := DeserializeOps(tc.in)
			require.NoError(t, err)
			assem := NewSmartOpAssembler()
			for _, op := range *ops {
				assem.Append(op)
			}
			assem.EndDocument()
			require.Equal(t, tc.want, assem.String())
		})
	}
}

// Upstream's deprecated appendOpWithText is opsFromText followed by append.
func TestSmartOpAssemblerAppendOpsFromText(t *testing.T) {
	type piece struct{ text, attribs string }
	cases := []struct {
		name   string
		pieces []piece
		want   string
	}{
		{
			"+ op with text",
			[]piece{{"test", "*3*4*5"}, {"test", "*3*4*5"}, {"test", "*1*4*5"}},
			"*3*4*5+8*1*4*5+4",
		},
		{
			"+ op with multiline text",
			[]piece{{"test\ntest", "*3*4*5"}, {"\ntest\n", "*3*4*5"}, {"\ntest", "*1*4*5"}},
			"*3*4*5|3+f*1*4*5|1+1*1*4*5+4",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assem := NewSmartOpAssembler()
			for _, p := range tc.pieces {
				attribs := p.attribs
				for _, op := range OpsFromText("+", p.text, &KeepArgs{stringAttribs: &attribs}, nil) {
					assem.Append(op)
				}
			}
			assem.EndDocument()
			require.Equal(t, tc.want, assem.String())
		})
	}
}
