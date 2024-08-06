package changeset

type Iterator[T any] struct {
	ops  []T
	next int
}

// Next returns the next element in the iterator and a boolean indicating if it's the last element
func (it *Iterator[T]) Next() (*T, bool) {
	if it.next >= len(it.ops) {
		return nil, true
	}
	op := it.ops[it.next]
	it.next++
	return &op, false
}
