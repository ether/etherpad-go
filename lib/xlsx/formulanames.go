package xlsx

import "strings"

// Excel stores every function added after 2007 under a namespace prefix in the
// file format: "_xlfn.XLOOKUP(...)", and "_xlfn._xlws.SORT(...)" for the
// worksheet-scoped dynamic-array ones. The prefix is invisible in Excel's UI
// and unknown to our formula engine, so it is stripped on import and added back
// on export — without it Excel shows #NAME? for formulas we wrote.

// xlwsFunctions need the worksheet-scoped prefix instead of the plain one.
var xlwsFunctions = map[string]bool{
	"SORT":   true,
	"FILTER": true,
}

// prefixedFunctions lists the non-dotted functions Excel namespaces. Dotted
// names (NORM.DIST, RANK.AVG, ...) are all post-2007 too and handled by rule.
var prefixedFunctions = map[string]bool{
	"ARABIC": true, "ARRAYTOTEXT": true, "BASE": true, "BITAND": true, "BITLSHIFT": true,
	"BITOR": true, "BITRSHIFT": true, "BITXOR": true, "CHOOSECOLS": true,
	"CHOOSEROWS": true, "COMBINA": true, "CONCAT": true, "COT": true, "COTH": true,
	"CSC": true, "CSCH": true, "DAYS": true, "DECIMAL": true, "DROP": true, "ENCODEURL": true,
	"EXPAND": true, "FILTER": true, "FORMULATEXT": true, "GAMMA": true, "GAUSS": true,
	"HSTACK": true, "IFNA": true, "IFS": true, "IMCOSH": true, "IMCOT": true, "IMCSC": true,
	"IMCSCH": true, "IMSEC": true, "IMSECH": true, "IMSINH": true, "IMTAN": true,
	"ISFORMULA": true, "ISOMITTED": true, "ISOWEEKNUM": true, "MAXIFS": true, "MINIFS": true,
	"MUNIT": true, "NUMBERVALUE": true, "PDURATION": true, "PERMUTATIONA": true, "PHI": true,
	"RANDARRAY": true, "RRI": true, "SEC": true, "SECH": true, "SEQUENCE": true, "SHEET": true,
	"SHEETS": true, "SORT": true, "SORTBY": true, "SWITCH": true, "TAKE": true,
	"TEXTAFTER": true, "TEXTBEFORE": true, "TEXTJOIN": true, "TEXTSPLIT": true, "TOCOL": true,
	"TOROW": true, "UNICHAR": true, "UNICODE": true, "UNIQUE": true, "VSTACK": true,
	"WEBSERVICE": true, "XLOOKUP": true, "XMATCH": true, "XOR": true,
}

func needsPrefix(name string) bool {
	if prefixedFunctions[name] {
		return true
	}
	// Dotted names are the 2010+ statistical/compatibility set (NORM.DIST,
	// MODE.SNGL, CEILING.MATH, ...), all of which Excel namespaces.
	return strings.Contains(name, ".")
}

// stripFunctionPrefixes removes Excel's namespace prefixes from a formula so
// our engine sees plain function names.
func stripFunctionPrefixes(formula string) string {
	for _, p := range []string{"_xlfn._xlws.", "_xlfn.", "_xlws."} {
		formula = strings.ReplaceAll(formula, p, "")
	}
	return formula
}

// addFunctionPrefixes namespaces the post-2007 function names in a formula.
// Function names are identifiers directly followed by '('; string literals are
// skipped so text like "SORT(" inside quotes is left alone.
func addFunctionPrefixes(formula string) string {
	var out strings.Builder
	for i := 0; i < len(formula); {
		c := formula[i]
		if c == '"' {
			j := i + 1
			for j < len(formula) {
				if formula[j] == '"' {
					// "" is an escaped quote inside the literal.
					if j+1 < len(formula) && formula[j+1] == '"' {
						j += 2
						continue
					}
					break
				}
				j++
			}
			if j < len(formula) {
				j++
			}
			out.WriteString(formula[i:j])
			i = j
			continue
		}
		if !isNameStart(c) {
			out.WriteByte(c)
			i++
			continue
		}
		j := i
		for j < len(formula) && isNameByte(formula[j]) {
			j++
		}
		name := formula[i:j]
		if j < len(formula) && formula[j] == '(' && needsPrefix(strings.ToUpper(name)) {
			if xlwsFunctions[strings.ToUpper(name)] {
				out.WriteString("_xlfn._xlws.")
			} else {
				out.WriteString("_xlfn.")
			}
		}
		out.WriteString(name)
		i = j
	}
	return out.String()
}

func isNameStart(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

func isNameByte(c byte) bool {
	return isNameStart(c) || c >= '0' && c <= '9' || c == '.' || c == '_'
}
