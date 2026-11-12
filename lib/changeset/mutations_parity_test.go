package changeset

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/stretchr/testify/require"
)

// The fixed vectors of etherpad-lite's easysync-mutations spec for
// MutateTextLines and MutateAttributionLines.

func TestMutateTextLinesVectors(t *testing.T) {
	cases := []struct {
		cs          string
		lines, want []string
	}{
		{"Z:4<1|1-2-1|1+1+1$\nc", []string{"a\n", "b\n"}, []string{"\n", "c\n"}},
		{"Z:4>0|1-2-1|2+3$\nc\n", []string{"a\n", "b\n"}, []string{"\n", "c\n", "\n"}},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("testMutateTextLines#%d", i+1), func(t *testing.T) {
			lines := append([]string{}, tc.lines...)
			require.NoError(t, MutateTextLines(tc.cs, &lines))
			require.Equal(t, tc.want, lines)
		})
	}

	t.Run("mutate keep only lines", func(t *testing.T) {
		lines := []string{"1\n", "2\n", "3\n", "4\n"}
		want := append([]string{}, lines...)
		require.NoError(t, MutateTextLines("Z:8>0*0|1=2|2=2", &lines))
		require.Equal(t, want, lines)
	})
}

// charPool is upstream's testPoolWithChars: 0 is char=newline, 1..35 are
// char=<base36 digit>, 36 is char="".
func charPool() apool.APool {
	p := apool.NewAPool()
	p.PutAttrib(apool.Attribute{Key: "char", Value: "newline"}, nil)
	for i := 1; i < 36; i++ {
		p.PutAttrib(apool.Attribute{Key: "char", Value: utils.NumToString(i)}, nil)
	}
	p.PutAttrib(apool.Attribute{Key: "char", Value: ""}, nil)
	return p
}

func TestMutateAttributionLinesVectors(t *testing.T) {
	// A line starting with "?" is not a valid attribution string: such lines
	// must be skipped without being parsed.
	cases := []struct {
		pool     func() apool.APool
		cs       string
		in, want []string
	}{
		// turn 123\n 456\n 789\n into 123\n 4<b>5</b>6\n 789\n
		{pairs("bold,true"), "Z:c>0|1=4=1*0=1$",
			[]string{"|1+4", "|1+4", "|1+4"},
			[]string{"|1+4", "+1*0+1|1+2", "|1+4"}},
		// make a document bold
		{pairs("bold,true"), "Z:c>0*0|3=c$",
			[]string{"|1+4", "|1+4", "|1+4"},
			[]string{"*0|1+4", "*0|1+4", "*0|1+4"}},
		// clear bold on document
		{pairs("bold,", "bold,true"), "Z:c>0*0|3=c$",
			[]string{"*1+1+1*1+1|1+1", "+1*1+1|1+2", "*1+1+1*1+1|1+1"},
			[]string{"|1+4", "|1+4", "|1+4"}},
		// a character on line 3 of 5; purely kept lines are not parsed
		{pairs("foo,bar", "line,1", "line,2", "line,3", "line,4", "line,5"), "Z:5>1|2=2+1$x",
			[]string{"?*1|1+1", "?*2|1+1", "*3|1+1", "?*4|1+1", "?*5|1+1"},
			[]string{"?*1|1+1", "?*2|1+1", "+1*3|1+1", "?*4|1+1", "?*5|1+1"}},
		// based on runMutationTest#1
		{charPool, "Z:11>7-2*t+1*u+1|2=b|2+a=2*b+1*o+1*t+1*0|1+1*b+1*u+1=3|1-3-6$tucream\npie\nbot\nbu",
			[]string{
				"*a+1*p+2*l+1*e+1*0|1+1",
				"*b+1*a+1*n+1*a+1*n+1*a+1*0|1+1",
				"*c+1*a+1*b+2*a+1*g+1*e+1*0|1+1",
				"*d+1*u+1*f+2*l+1*e+1*0|1+1",
				"*e+1*g+2*p+1*l+1*a+1*n+1*t+1*0|1+1",
			},
			[]string{
				"*t+1*u+1*p+1*l+1*e+1*0|1+1",
				"*b+1*a+1*n+1*a+1*n+1*a+1*0|1+1",
				"|1+6",
				"|1+4",
				"*c+1*a+1*b+1*o+1*t+1*0|1+1",
				"*b+1*u+1*b+2*a+1*0|1+1",
				"*e+1*g+2*p+1*l+1*a+1*n+1*t+1*0|1+1",
			}},
		// based on runMutationTest#3
		{charPool, "Z:11<f|1-6|2=f=6|1-1-8$",
			[]string{"*a|1+6", "*b|1+7", "*c|1+8", "*d|1+7", "*e|1+9"},
			[]string{"*b|1+7", "*c|1+8", "*d+6*e|1+1"}},
		// based on runMutationTest#4
		{charPool, "Z:3>7=1|4+7$\n2\n3\n4\n",
			[]string{"*1+1*5|1+2"},
			[]string{"*1+1|1+1", "|1+2", "|1+2", "|1+2", "*5|1+2"}},
		// based on runMutationTest#5
		{charPool, "Z:a<7=1|4-7$",
			[]string{"*1|1+2", "*2|1+2", "*3|1+2", "*4|1+2", "*5|1+2"},
			[]string{"*1+1*5|1+2"}},
		// based on runMutationTest#6
		{charPool, "Z:k<7*0+1*10|2=8|2-8$0",
			[]string{
				"*1+1*2+1*3+1|1+1",
				"*a+1*b+1*c+1|1+1",
				"*d+1*e+1*f+1|1+1",
				"*g+1*h+1*i+1|1+1",
				"?*x+1*y+1*z+1|1+1",
			},
			[]string{"*0+1|1+4", "|1+4", "?*x+1*y+1*z+1|1+1"}},
		{charPool, "Z:6>4=1+1=1+1|1=1+1=1*0+1$abcd",
			[]string{"|1+3", "|1+3"},
			[]string{"|1+5", "+2*0+1|1+2"}},
		{charPool, "Z:s>1|1=4=6|1+1$\n",
			[]string{"*0|1+4", "*0|1+8", "*0+5|1+1", "*0|1+1", "*0|1+5", "*0|1+1", "*0|1+1", "*0|1+1", "|1+1"},
			[]string{"*0|1+4", "*0+6|1+1", "*0|1+2", "*0+5|1+1", "*0|1+1", "*0|1+5", "*0|1+1", "*0|1+1", "*0|1+1", "|1+1"}},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("runMutateAttributionTest#%d", i+1), func(t *testing.T) {
			pool := tc.pool()
			cs, err := CheckRep(tc.cs)
			require.NoError(t, err)

			lines := append([]string{}, tc.in...)
			require.NoError(t, MutateAttributionLines(*cs, &lines, &pool))
			require.Equal(t, tc.want, lines)

			// The same change applied to the joined attribution string.
			merged, err := ApplyToAttribution(tc.cs, JoinAttributionLines(noMarks(tc.in)), tc.pool())
			require.NoError(t, err)
			require.Equal(t, JoinAttributionLines(noMarks(tc.want)), *merged)
		})
	}
}

func pairs(attribs ...string) func() apool.APool {
	return func() apool.APool {
		p := apool.NewAPool()
		for _, a := range attribs {
			k, v, _ := strings.Cut(a, ",")
			p.PutAttrib(apool.Attribute{Key: k, Value: v}, nil)
		}
		return p
	}
}

func noMarks(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strings.ReplaceAll(l, "?", "")
	}
	return out
}
