package changeset

const lineMarkerAttribute = "lmkr"

// Some of these attributes are kept for compatibility purposes.
// Not sure if we need all of them
var DEFAULT_LINE_ATTRIBUTES = []string{
	"author", "lmkr", "insertorder", "start",
}

// If one of these attributes are set to the first character of a
// line it is considered as a line attribute marker i.e. attributes
// set on this marker are applied to the whole line.
// The list attribute is only maintained for compatibility reasons
var LineAttributes = []string{lineMarkerAttribute, "list"}

/*
  The Attribute manager builds changesets based on a document
  representation for setting and removing range or line-based attributes.

  @param rep the document representation to be used
  @param applyChangesetCallback this callback will be called
    once a changeset has been built.


  A document representation contains
  - an array `alines` containing 1 attributes string for each line
  - an Attribute pool `apool`
  - a SkipList `lines` containing the text lines of the document.
*/

type AttributeManager struct {
}

func HasAttrib(attribs *AttributeMap) bool {
	for _, a := range LineAttributes {
		if exists := attribs.Has(a); exists {
			return true
		}
	}
	return false
}
