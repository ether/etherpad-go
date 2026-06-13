package events

// The pad-lifecycle hook contexts. The engine pad object is exposed as `any`
// to avoid the lib/models/pad -> lib/hooks import cycle; plugins type-assert it
// to *pad.Pad (plugins are leaf packages and may import lib/models/pad).

// PadDefaultContentContext is passed to the padDefaultContent hook before a new
// pad's initial revision is written. A callback may replace Content (and Type);
// the caller reads Content back after the hook runs.
type PadDefaultContentContext struct {
	Type     *string
	Content  *string
	Pad      any
	AuthorId *string
	PadId    string
}

// PadLoadContext is passed to the padLoad hook whenever a pad is materialized.
type PadLoadContext struct {
	Pad   any
	PadId string
}

// PadCreateContext is passed to the padCreate hook right after a pad's first
// revision is persisted.
type PadCreateContext struct {
	Pad   any
	PadId string
	// AuthorId is the creating author; empty string when the pad is created
	// without a known author (e.g. server-side operations). Unlike
	// PadDefaultContentContext.AuthorId this is a plain string, not a pointer.
	AuthorId string
}

// PadUpdateContext is passed to the padUpdate hook after a revision is appended.
type PadUpdateContext struct {
	Pad      any
	PadId    string
	AuthorId string
	// Revs is the pad's new head revision number after this update.
	Revs      int
	Changeset string
}

// PadCopyContext is passed to the padCopy hook after a pad is copied to a new
// destination (copyPad, copyPadWithoutHistory, movePad).
type PadCopyContext struct {
	SrcPad any
	DstPad any
	SrcId  string
	DstId  string
}

// PadRemoveContext is passed to the padRemove hook when a pad is deleted.
type PadRemoveContext struct {
	Pad   any
	PadId string
}
