package sheet

// Transform adjusts `in` so it applies cleanly after `applied`, where both were
// originally composed against the same base revision and `applied` was ordered
// first by the server. Only structural ops (row/col insert/delete) on the same
// sheet and axis move coordinates; everything else is returned unchanged.
func Transform(in, applied Op) Op {
	if in.Sheet != applied.Sheet || !applied.isStructural() {
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

// shiftRows moves row coordinates of `in` by delta for rows at/after index.
// delta > 0 is an insert; delta < 0 is a delete (band [index, index-delta)).
func shiftRows(in Op, index, delta int) Op {
	in.Row = shiftCoord(in.Row, index, delta)
	if in.Type == OpClearRange {
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
	if in.Type == OpClearRange {
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
