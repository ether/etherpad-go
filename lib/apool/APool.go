package apool

import (
	"errors"
)

type APool struct {
	NumToAttrib map[int]Attribute
	AttribToNum map[Attribute]int
	nextNum     int
}

func NewAPool() *APool {
	return &APool{
		NumToAttrib: make(map[int]Attribute),
		AttribToNum: make(map[Attribute]int),
		nextNum:     0,
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

	a.nextNum++
	a.AttribToNum[attrib] = a.nextNum
	a.NumToAttrib[a.nextNum] = attrib

	return a.nextNum
}

/**
 * Replace the contents of this attribute pool with values from a previous call to `toJsonable`.
 *
 * @param {Jsonable} obj - Object returned by `toJsonable` containing the attributes and their
 *     identifiers. WARNING: This function takes ownership of the object (it does not make a deep
 *     copy). Use the `clone()` method to copy a pool -- do NOT do
 *     `new AttributePool().fromJsonable(pool.toJsonable())` to copy because the resulting shared
 *     state will lead to pool corruption.
 */
func (a *APool) fromJsonable(obj APool) *APool {
	a.NumToAttrib = obj.NumToAttrib
	a.AttribToNum = make(map[Attribute]int)
	a.nextNum = obj.nextNum

	for num, attrib := range a.NumToAttrib {
		a.AttribToNum[attrib] = num
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
func (a *APool) toJsonable() APool {
	return APool{
		NumToAttrib: a.NumToAttrib,
		nextNum:     a.nextNum,
	}
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

	newPool.nextNum = a.nextNum
	return newPool
}

/**
 * Asserts that the data in the pool is consistent. Throws if inconsistent.
 */
func (a *APool) check() error {
	if a.nextNum < 0 {
		return errors.New("nextNum is negative")
	}
	if len(a.AttribToNum) != a.nextNum {
		return errors.New("nextNum is not equal to the number of attributes")
	}
	if len(a.NumToAttrib) != a.nextNum {
		return errors.New("nextNum is not equal to the number of attributes")
	}

	for i := 0; i < a.nextNum; i++ {
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

func (a *APool) GetAttrib(num int) Attribute {
	pair, ok := a.NumToAttrib[num]
	if !ok {
		return pair
	}
	return pair
}
