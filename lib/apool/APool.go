package apool

import (
	"errors"
)

type APool struct {
	NumToAttrib    map[int]Attribute `json:"-"`
	NumToAttribRaw map[int][]string  `json:"numToAttrib"`
	AttribToNum    map[Attribute]int `json:"-"`
	NextNum        int               `json:"nextNum"`
}

func NewAPool() APool {
	return APool{
		NumToAttrib: make(map[int]Attribute),
		AttribToNum: make(map[Attribute]int),
		NextNum:     0,
	}
}

type EachAttribFunc func(attrib Attribute)

func (a *APool) EachAttrib(f EachAttribFunc) {
	for _, attrib := range a.NumToAttrib {
		f(attrib)
	}
}

func (a *APool) PutAttrib(attrib Attribute, dontAddIfAbsent *bool) int {
	var val, ok = a.AttribToNum[attrib]
	if ok {
		return val
	}

	if dontAddIfAbsent != nil && *dontAddIfAbsent {
		return -1
	}

	var num = a.NextNum
	a.NextNum++
	a.AttribToNum[attrib] = num
	a.NumToAttrib[num] = attrib

	return num
}

// FromJsonable /**
func (a *APool) FromJsonable(obj APool) *APool {
	a.AttribToNum = make(map[Attribute]int)
	a.NextNum = obj.NextNum
	a.NumToAttribRaw = obj.NumToAttribRaw
	for num, attrib := range obj.NumToAttribRaw {
		var entry = FromJsonAble(attrib)
		a.NumToAttrib[num] = entry
		a.AttribToNum[entry] = num
	}

	return a
}

/**
 * @returns {Jsonable} An object that can be passed to `fromJsonable` to reconstruct this
 *     attribute pool. The returned object can be converted to JSON. WARNING: The returned object
 *     has references to internal state (it is not a deep copy). Use the `clone()` method to copy
 *     a pool -- do NOT do `new AttributePool().fromJsonable(pool.toJsonable())` to copy because
 *     the resulting shared state will lead to pool corruption.
 */
func (a *APool) ToJsonable() APool {
	var jsonAbleMap = make(map[int][]string)
	for s := range a.NumToAttrib {
		var attrib = a.NumToAttrib[s]
		var entry = attrib.ToJsonAble()
		jsonAbleMap[s] = entry
	}
	a.NumToAttribRaw = jsonAbleMap
	return *a
}

func (a *APool) clone() APool {
	var newPool = APool{}

	for num, attrib := range a.NumToAttrib {
		newPool.NumToAttrib[num] = attrib
		newPool.AttribToNum[attrib] = num
	}

	for attrib, num := range a.AttribToNum {
		newPool.AttribToNum[attrib] = num
		newPool.NumToAttrib[num] = attrib
	}

	newPool.NextNum = a.NextNum
	return newPool
}

/**
 * Asserts that the data in the pool is consistent. Throws if inconsistent.
 */
func (a *APool) check() error {
	if a.NextNum < 0 {
		return errors.New("nextNum is negative")
	}
	if len(a.AttribToNum) != a.NextNum {
		return errors.New("nextNum is not equal to the number of attributes")
	}
	if len(a.NumToAttrib) != a.NextNum {
		return errors.New("nextNum is not equal to the number of attributes")
	}

	for i := 0; i < a.NextNum; i++ {
		if _, ok := a.NumToAttrib[i]; !ok {
			return errors.New("attribute not found")
		}
	}
	return nil
}

type AttributeIterator func(attributeKey *string, attributeValue *string)

/**
 * Executes a callback for each attribute in the pool.
 *
 * @param {Function} func - Callback to call with two arguments: key and value. Its return value
 *     is ignored.
 */
func (a *APool) eachAttrib(attribConv AttributeIterator) {
	for _, attrib := range a.NumToAttrib {
		attribConv(&attrib.Key, &attrib.Value)
	}
}

func (a *APool) GetAttrib(num int) (*Attribute, error) {
	pair, ok := a.NumToAttrib[num]
	if !ok {
		return nil, errors.New("attrib not found")
	}
	return &pair, nil
}
