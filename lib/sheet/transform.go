package sheet

// Transform adjusts `in` so it applies cleanly after `applied`, where both were
// originally composed against the same base revision and `applied` was ordered
// first by the server. Only structural ops (row/col insert/delete) on the same
// sheet and axis move coordinates; everything else is returned unchanged.
func Transform(in, applied Op) Op {
	in = rebaseFormula(in, applied)
	if in.Sheet != applied.Sheet || !applied.isStructural() {
		return in
	}
	// A filter on a column deleted meanwhile has nothing left to filter;
	// shifting it would put it on the column that moved into its place.
	if in.Type == OpSetFilter && !in.Clear && applied.Type == OpDeleteCols &&
		in.Col >= applied.Index && in.Col < applied.Index+applied.Count {
		in.Noop = true
		return in
	}
	switch applied.Type {
	case OpInsertRows:
		return shiftRows(in, applied.Index, applied.Count)
	case OpDeleteRows:
		return shiftRows(in, applied.Index, -applied.Count)
	case OpInsertCols:
		return shiftCols(in, applied.Index, applied.Count)
	case OpDeleteCols:
		return shiftCols(in, applied.Index, -applied.Count)
	}
	return in
}

// rebaseFormula changes a formula typed before a concurrent op the way Apply
// changed the formulas already stored: a row/column insert or delete moves
// its references (those without a sheet name only on the same sheet), a
// rename renames them, a sheet delete breaks them. The sheet names come from
// the applied op (FillSheetNames); ops stored before it carry none and only
// move references without a sheet name.
func rebaseFormula(in, applied Op) Op {
	if in.Type != OpSetCell || in.Raw == nil || len(*in.Raw) == 0 || (*in.Raw)[0] != '=' {
		return in
	}
	e, ok := refEditOf(applied)
	if !ok {
		return in
	}
	e.OnTarget = in.Sheet == applied.Sheet
	if raw := RewriteFormula(*in.Raw, e); raw != *in.Raw {
		in.Raw = &raw
	}
	return in
}

// refEditOf is the reference edit an op makes, if any.
func refEditOf(op Op) (RefEdit, bool) {
	switch op.Type {
	case OpInsertRows, OpDeleteRows, OpInsertCols, OpDeleteCols:
		e := RefEdit{Target: op.Name, Index: op.Index, Delta: op.Count, Axis: "row"}
		if op.Type == OpInsertCols || op.Type == OpDeleteCols {
			e.Axis = "col"
		}
		if op.Type == OpDeleteRows || op.Type == OpDeleteCols {
			e.Delta = -op.Count
		}
		return e, true
	case OpRenameSheet:
		if op.OldName == "" || op.OldName == op.Name {
			return RefEdit{}, false
		}
		return RefEdit{Target: op.OldName, Rename: op.Name}, true
	case OpDeleteSheet:
		if op.Name == "" {
			return RefEdit{}, false
		}
		return RefEdit{Target: op.Name, Deleted: true}, true
	}
	return RefEdit{}, false
}

// FillSheetNames records in op the sheet names Transform needs to move
// formulas typed concurrently: the sheet's name on row/column inserts and
// deletes and on deleteSheet, the old name on renameSheet. Called on the
// server before an op is applied and stored; what the client sent there is
// replaced.
func (w *Workbook) FillSheetNames(op *Op) {
	s := w.SheetByID(op.Sheet)
	switch op.Type {
	case OpInsertRows, OpDeleteRows, OpInsertCols, OpDeleteCols, OpDeleteSheet:
		op.Name = ""
		if s != nil {
			op.Name = s.Name
		}
	}
	op.OldName = ""
	if op.Type == OpRenameSheet && s != nil {
		op.OldName = s.Name
	}
}

// shiftRows moves row coordinates of `in` by delta for rows at/after index.
// delta > 0 is an insert; delta < 0 is a delete (band [index, index-delta)).
// hasRange reports whether the op carries an EndRow/EndCol rectangle.
func hasRange(t OpType) bool {
	return t == OpClearRange || t == OpMergeCells || t == OpUnmergeCells
}

func shiftRows(in Op, index, delta int) Op {
	in.Row = shiftCoord(in.Row, index, delta)
	if hasRange(in.Type) {
		in.EndRow = shiftCoord(in.EndRow, index, delta)
	}
	if in.Type == OpInsertRows || in.Type == OpDeleteRows {
		in.Index = shiftCoord(in.Index, index, delta)
	}
	if in.Type == OpSetDimension && in.Axis == "row" {
		in.Index = shiftCoord(in.Index, index, delta)
	}
	return in
}

func shiftCols(in Op, index, delta int) Op {
	in.Col = shiftCoord(in.Col, index, delta)
	if hasRange(in.Type) {
		in.EndCol = shiftCoord(in.EndCol, index, delta)
	}
	if in.Type == OpInsertCols || in.Type == OpDeleteCols {
		in.Index = shiftCoord(in.Index, index, delta)
	}
	if in.Type == OpSetDimension && in.Axis == "col" {
		in.Index = shiftCoord(in.Index, index, delta)
	}
	return in
}

// shiftCoord shifts a single coordinate. For inserts (delta>0) coords at/after
// index move right/down. For deletes (delta<0) coords after the band move back;
// coords inside the deleted band clamp to index.
func shiftCoord(coord, index, delta int) int {
	if delta >= 0 {
		if coord >= index {
			return coord + delta
		}
		return coord
	}
	band := -delta
	if coord < index {
		return coord
	}
	if coord < index+band {
		return index // inside deleted band: clamp
	}
	return coord - band
}
