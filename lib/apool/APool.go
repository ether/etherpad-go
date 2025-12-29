package apool

import (
	"errors"
	"strconv"

	"github.com/ether/etherpad-go/lib/models/db"
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

func (a *APool) ToRevDB() db.RevPool {

	attribNums := make(map[string]int)
	numToAttrib := make(map[string][]string)
	for attrib, num := range a.AttribToNum {
		attribNums[attrib.String()] = num
	}

	for num, attrib := range a.NumToAttrib {
		numToAttrib[strconv.Itoa(num)] = attrib.ToStringSlice()
	}

	var dbPool = db.RevPool{
		NextNum:     a.NextNum,
		AttribToNum: attribNums,
		NumToAttrib: numToAttrib,
	}
	return dbPool
}

func (a *APool) ToPadDB() db.PadPool {

	numToAttrib := make(map[string][]string)
	for num, attrib := range a.NumToAttrib {
		numToAttrib[strconv.Itoa(num)] = attrib.ToStringSlice()
	}

	var dbPool = db.PadPool{
		NextNum:     a.NextNum,
		NumToAttrib: numToAttrib,
	}
	return dbPool
}

func (a *APool) Check() error {
	if a.NextNum < 0 {
		return errors.New("nextNum is negative")
	}

	if len(a.NumToAttrib) != a.NextNum {
		return errors.New("numToAttrib length does not match nextNum")
	}

	if len(a.AttribToNum) != a.NextNum {
		return errors.New("attribToNum length does not match nextNum")
	}
	for i := 0; i < a.NextNum; i++ {
		attr, ok := a.NumToAttrib[i]
		if !ok {
			return errors.New("numToAttrib missing entry for index " + string(rune(i)))
		}
		if attr.Key == "" {
			return errors.New("attribute key is empty for index " + string(rune(i)))
		}
		if a.AttribToNum[attr] != i {
			return errors.New("attribToNum mapping incorrect for attribute " + attr.Key + ":" + attr.Value)
		}
	}
	return nil
}

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

func (a *APool) FromDB(obj db.PadPool) *APool {
	a.AttribToNum = make(map[Attribute]int)
	a.NextNum = obj.NextNum
	a.NumToAttribRaw = obj.ToIntPool()
	for num, attrib := range a.NumToAttribRaw {
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

func (a *APool) toDBRev() db.RevPool {
	numToAttrib := make(map[string][]string)
	for num, attrib := range a.NumToAttrib {
		numToAttrib[strconv.Itoa(num)] = attrib.ToStringSlice()
	}

	attribToNum := make(map[string]int)
	for attrib, num := range a.AttribToNum {
		attribToNum[attrib.String()] = num
	}

	return db.RevPool{
		NumToAttrib: numToAttrib,
		NextNum:     a.NextNum,
		AttribToNum: attribToNum,
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

	newPool.NextNum = a.NextNum
	return newPool
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
