package changeset

import "github.com/ether/etherpad-go/lib/apool"

type Builder struct {
	oldLen   int
	assem    SmartOpAssembler
	o        Op
	charBank StringAssembler
}

func NewBuilder(oldLen int) Builder {
	return Builder{
		oldLen:   oldLen,
		assem:    *NewSmartOpAssembler(),
		o:        NewOp(nil),
		charBank: NewStringAssembler(),
	}
}

type KeepArgs struct {
	stringAttribs *string
	apoolAttribs  *[]apool.Attribute
}

/**
 * @param {number} N - Number of characters to keep.
 * @param {number} L - Number of newlines among the `N` characters. If positive, the last
 *     character must be a newline.
 * @param {(string|Attribute[])} attribs - Either [[key1,value1],[key2,value2],...] or '*0*1...'
 *     (no pool needed in latter case).
 * @param {?AttributePool} pool - Attribute pool, only required if `attribs` is a list of
 *     attribute key, value pairs.
 * @returns {Builder} this
 */
func (b Builder) Keep(N int, L int, attribs KeepArgs, pool *apool.APool) Builder {
	b.o.OpCode = "="

	if attribs.stringAttribs != nil {
		b.o.Attribs = *attribs.stringAttribs
	} else {
		attributeMap := NewAttributeMap(pool)
		if attribs.apoolAttribs == nil {
			attribs.apoolAttribs = &[]apool.Attribute{}
		}
		var updatedMap = attributeMap.Update(*attribs.apoolAttribs, nil)
		b.o.Attribs = updatedMap.String()
	}

	b.o.Chars = N
	if L > 0 {
		b.o.Lines = L
	} else {
		b.o.Lines = 0
	}

	b.assem.Append(b.o)

	return b
}

/**
 * @param {string} text - Text to keep.
 * @param {(string|Attribute[])} attribs - Either [[key1,value1],[key2,value2],...] or '*0*1...'
 *     (no pool needed in latter case).
 * @param {?AttributePool} pool - Attribute pool, only required if `attribs` is a list of
 *     attribute key, value pairs.
 * @returns {Builder} this
 */
func (b Builder) KeepText(text string, attribs *KeepArgs, pool *apool.APool) Builder {
	for _, op := range OpsFromText("=", text, attribs, pool) {
		b.assem.Append(op)
	}
	return b
}

/**
 * @param {string} text - Text to insert.
 * @param {(string|Attribute[])} attribs - Either [[key1,value1],[key2,value2],...] or '*0*1...'
 *     (no pool needed in latter case).
 * @param {?AttributePool} pool - Attribute pool, only required if `attribs` is a list of
 *     attribute key, value pairs.
 * @returns {Builder} this
 */
func (b Builder) Insert(text string, attribs KeepArgs, pool *apool.APool) Builder {
	for _, op := range OpsFromText("+", text, &attribs, pool) {
		b.assem.Append(op)
	}
	b.charBank.Append(text)

	return b
}

func (b Builder) Remove(N, L int) Builder {
	b.o.OpCode = "-"
	b.o.Attribs = ""
	b.o.Chars = N
	if L > 0 {
		b.o.Lines = L
	} else {
		b.o.Lines = 0
	}
	b.assem.Append(b.o)
	return b
}

func (b Builder) ToString() string {
	b.assem.EndDocument()
	var newLen = b.oldLen + b.assem.LengthChange()
	return Pack(b.oldLen, newLen, b.assem.String(), b.charBank.String())
}
