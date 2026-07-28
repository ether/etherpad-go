package xlsx

import "testing"

func TestAddFunctionPrefixes(t *testing.T) {
	cases := []struct{ in, want string }{
		{"SUM(A1:A2)", "SUM(A1:A2)"},
		{"IF(A1>0,1,2)", "IF(A1>0,1,2)"},
		{"XLOOKUP(A1,B:B,C:C)", "_xlfn.XLOOKUP(A1,B:B,C:C)"},
		{"SORT(A1:A4)", "_xlfn._xlws.SORT(A1:A4)"},
		{"UNIQUE(SORT(A1:A4))", "_xlfn.UNIQUE(_xlfn._xlws.SORT(A1:A4))"},
		{"RANK.AVG(A1,B1:B4)", "_xlfn.RANK.AVG(A1,B1:B4)"},
		{"NORM.DIST(1,0,1,TRUE)", "_xlfn.NORM.DIST(1,0,1,TRUE)"},
		// Text literals must not be rewritten, even when they look like calls.
		{`CONCAT("SORT(","x")`, `_xlfn.CONCAT("SORT(","x")`},
		{`IF(A1="UNIQUE(",1,2)`, `IF(A1="UNIQUE(",1,2)`},
		// Sheet-qualified references stay untouched.
		{"SUM(Sheet2!A1:A2)", "SUM(Sheet2!A1:A2)"},
	}
	for _, c := range cases {
		if got := addFunctionPrefixes(c.in); got != c.want {
			t.Errorf("addFunctionPrefixes(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStripFunctionPrefixesRoundTrip(t *testing.T) {
	for _, f := range []string{"SUM(A1:A2)", "_xlfn.UNIQUE(_xlfn._xlws.SORT(A1:A4))", "_xlfn.TEXTBEFORE(A1,\"-\")"} {
		stripped := stripFunctionPrefixes(f)
		if got := addFunctionPrefixes(stripped); got != f {
			t.Errorf("round trip of %q gave %q", f, got)
		}
	}
	if got := stripFunctionPrefixes("_xlws.FILTER(A1:A4,B1:B4)"); got != "FILTER(A1:A4,B1:B4)" {
		t.Errorf("stripFunctionPrefixes dropped the wrong part: %q", got)
	}
}
