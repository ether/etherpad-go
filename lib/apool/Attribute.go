package apool

type Attribute struct {
	Key   string
	Value string
}

func CmpAttribute(a, b Attribute) int {
	if a.Key < b.Key {
		return -1
	}
	if a.Key > b.Key {
		return 1
	}
	if a.Value < b.Value {
		return -1
	}
	if a.Value > b.Value {
		return 1
	}
	return 0
}
