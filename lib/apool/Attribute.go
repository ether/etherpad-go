package apool

type Attribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
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

func (a *Attribute) ToJsonAble() []string {
	var result = make([]string, 2)
	result[0] = a.Key
	result[1] = a.Value
	return result
}

func FromJsonAble(convertable []string) Attribute {
	return Attribute{
		Key:   convertable[0],
		Value: convertable[1],
	}
}
