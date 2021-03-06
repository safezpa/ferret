package values

import (
	"encoding/binary"
	"encoding/json"
	"hash/fnv"
	"reflect"
	"sort"
	"time"

	"github.com/MontFerret/ferret/pkg/runtime/core"
)

func GetIn(from core.Value, byPath []core.Value) (core.Value, error) {
	if byPath == nil || len(byPath) == 0 {
		return None, nil
	}

	var result = from
	var err error

	for _, segment := range byPath {
		if result == None || result == nil {
			break
		}

		segmentType := segment.Type()

		switch result.Type() {
		case core.ObjectType:
			obj := result.(*Object)

			if segmentType != core.StringType {
				return nil, core.TypeError(segmentType, core.StringType)
			}

			result, _ = obj.Get(segment.(String))

			break
		case core.ArrayType:
			arr := result.(*Array)

			if segmentType != core.IntType {
				return nil, core.TypeError(segmentType, core.IntType)
			}

			result = arr.Get(segment.(Int))

			break
		case core.HTMLElementType, core.HTMLDocumentType:
			el := result.(HTMLNode)

			if segmentType == core.IntType {
				result = el.GetChildNode(segment.(Int))
			} else if segmentType == core.StringType {
				strSegment := segment.(String)

				switch strSegment {
				case "nodeType":
					result = el.NodeType()
				case "nodeName":
					result = el.NodeName()
				case "innerText":
					result = el.InnerText()
				case "innerHTML":
					result = el.InnerHTML()
				case "value":
					result = el.Value()
				case "attributes":
					result = el.GetAttributes()
				case "children":
					result = el.GetChildNodes()
				case "length":
					result = el.Length()
				case "url":
					if result.Type() == core.HTMLDocumentType {
						doc, ok := result.(HTMLDocument)

						if ok {
							result = doc.URL()
						}
					}
				default:
					result = None
				}

				if err != nil {
					return None, err
				}
			} else {
				return nil, core.TypeError(segmentType, core.IntType, core.StringType)
			}

		default:
			return None, core.TypeError(
				from.Type(),
				core.ArrayType,
				core.ObjectType,
				core.HTMLDocumentType,
				core.HTMLElementType,
			)
		}
	}

	return result, nil
}

func SetIn(to core.Value, byPath []core.Value, value core.Value) error {
	if byPath == nil || len(byPath) == 0 {
		return nil
	}

	var parent core.Value
	var current = to
	target := len(byPath) - 1

	for idx, segment := range byPath {
		parent = current
		isTarget := target == idx
		segmentType := segment.Type()

		switch parent.Type() {
		case core.ObjectType:
			parent := parent.(*Object)

			if segmentType != core.StringType {
				return core.TypeError(segmentType, core.StringType)
			}

			if isTarget == false {
				current, _ = parent.Get(segment.(String))
			} else {
				parent.Set(segment.(String), value)
			}

			break
		case core.ArrayType:
			if segmentType != core.IntType {
				return core.TypeError(segmentType, core.IntType)
			}

			parent := parent.(*Array)

			if isTarget == false {
				current = parent.Get(segment.(Int))
			} else {
				parent.Set(segment.(Int), value)
			}

			break
		default:
			// redefine parent
			isArray := segmentType == core.IntType

			// it's not an index
			if isArray == false {
				obj := NewObject()
				parent = obj

				if segmentType != core.StringType {
					return core.TypeError(segmentType, core.StringType)
				}

				if isTarget {
					obj.Set(segment.(String), value)
				}
			} else {
				arr := NewArray(10)
				parent = arr

				if isTarget {
					arr.Set(segment.(Int), value)
				}
			}

			// set new parent
			SetIn(to, byPath[0:idx-1], parent)

			if isTarget == false {
				current = None
			}
		}
	}

	return nil
}

func Parse(input interface{}) core.Value {
	switch input.(type) {
	case bool:
		return NewBoolean(input.(bool))
	case string:
		return NewString(input.(string))
	case int:
		return NewInt(input.(int))
	case float64:
		return NewFloat(input.(float64))
	case float32:
		return NewFloat(float64(input.(float32)))
	case time.Time:
		return NewDateTime(input.(time.Time))
	case []interface{}:
		input := input.([]interface{})
		arr := NewArray(len(input))

		for _, el := range input {
			arr.Push(Parse(el))
		}

		return arr
	case map[string]interface{}:
		input := input.(map[string]interface{})
		obj := NewObject()

		for key, el := range input {
			obj.Set(NewString(key), Parse(el))
		}

		return obj
	case []byte:
		return NewBinary(input.([]byte))
	case nil:
		return None
	default:
		v := reflect.ValueOf(input)
		t := reflect.TypeOf(input)
		kind := t.Kind()

		if kind == reflect.Slice || kind == reflect.Array {
			size := v.Len()
			arr := NewArray(size)

			for i := 0; i < size; i++ {
				value := v.Index(i)
				arr.Push(Parse(value.Interface()))
			}

			return arr
		}

		if kind == reflect.Map {
			keys := v.MapKeys()
			obj := NewObject()

			for _, k := range keys {
				key := Parse(k.Interface())
				value := v.MapIndex(k)

				obj.Set(NewString(key.String()), Parse(value.Interface()))
			}

			return obj
		}

		if kind == reflect.Struct {
			obj := NewObject()
			size := t.NumField()

			for i := 0; i < size; i++ {
				field := t.Field(i)
				value := v.Field(i)

				obj.Set(NewString(field.Name), Parse(value.Interface()))
			}

			return obj
		}

		return None
	}
}

func Unmarshal(value json.RawMessage) (core.Value, error) {
	var o interface{}

	err := json.Unmarshal(value, &o)

	if err != nil {
		return None, err
	}

	return Parse(o), nil
}

func IsCloneable(value core.Value) Boolean {
	switch value.Type() {
	case core.ArrayType:
		return NewBoolean(true)
	case core.ObjectType:
		return NewBoolean(true)
	default:
		return NewBoolean(false)
	}
}

func ToBoolean(input core.Value) core.Value {
	switch input.Type() {
	case core.BooleanType:
		return input
	case core.NoneType:
		return False
	case core.StringType:
		return NewBoolean(input.String() != "")
	case core.IntType:
		return NewBoolean(input.(Int) != 0)
	case core.FloatType:
		return NewBoolean(input.(Float) != 0)
	default:
		return True
	}
}

func ToArray(input core.Value) core.Value {
	switch input.Type() {
	case core.BooleanType,
		core.IntType,
		core.FloatType,
		core.StringType,
		core.DateTimeType:

		return NewArrayWith(input)
	case core.HTMLElementType,
		core.HTMLDocumentType:
		val := input.(HTMLNode)
		attrs := val.GetAttributes()

		obj, ok := attrs.(*Object)

		if !ok {
			return NewArray(0)
		}

		arr := NewArray(int(obj.Length()))

		obj.ForEach(func(value core.Value, key string) bool {
			arr.Push(value)

			return true
		})

		return obj
	case core.ArrayType:
		return input.Copy()
	case core.ObjectType:
		obj, ok := input.(*Object)

		if !ok {
			return NewArray(0)
		}

		arr := NewArray(int(obj.Length()))

		obj.ForEach(func(value core.Value, key string) bool {
			arr.Push(value)

			return true
		})

		return obj
	default:
		return NewArray(0)
	}
}

func MapHash(input map[string]core.Value) uint64 {
	h := fnv.New64a()

	keys := make([]string, 0, len(input))

	for key := range input {
		keys = append(keys, key)
	}

	// order does not really matter
	// but it will give us a consistent hash sum
	sort.Strings(keys)
	endIndex := len(keys) - 1

	h.Write([]byte("{"))

	for idx, key := range keys {
		h.Write([]byte(key))
		h.Write([]byte(":"))

		el := input[key]

		bytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(bytes, el.Hash())

		h.Write(bytes)

		if idx != endIndex {
			h.Write([]byte(","))
		}
	}

	h.Write([]byte("}"))

	return h.Sum64()
}
