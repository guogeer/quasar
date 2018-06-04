package util

import (
	. "reflect"
	"github.com/guogeer/husky/log"
)

// deep copy from struct to struct
func DeepCopy(dst, src interface{}) {
	// log.Debugf("%#v %#v", dst, src)
	if dst == nil || src == nil {
		log.Info("nil value")
		return
	}

	srcVal := Indirect(ValueOf(src))
	dstVal := Indirect(ValueOf(dst))
	if !(srcVal.Kind() == Struct && dstVal.Kind() == Struct) {
		log.Error("type is not struct ptr")
		return
	}

	srcType := srcVal.Type()
	// dstType := dstVal.Type()
	for i := 0; i < srcVal.NumField(); i++ {
		srcField := srcVal.Field(i)
		name := srcType.Field(i).Name
		dstField := dstVal.FieldByName(name)
		if !dstField.CanSet() {
			continue
		}

		if getKind(srcField) != getKind(dstField) {
			continue
		}
		switch getKind(srcField) {
		case Int64:
			dstField.SetInt(srcField.Int())
		case Uint64:
			dstField.SetUint(srcField.Uint())
		case Float64:
			dstField.SetFloat(srcField.Float())
		case Bool, String:
			dstField.Set(srcField)
		case Ptr:
			DeepCopy(dstField.Interface(), srcField.Interface())
		default:
			// log.Infof("%#v %#v", srcField, dstField)
		}
	}
}

func getKind(v Value) Kind {
	kind := v.Kind()
	switch kind {
	case Int, Int8, Int16, Int32, Int64:
		return Int64
	case Uint, Uint8, Uint16, Uint32, Uint64:
		return Uint64
	case Float32, Float64:
		return Float64
	default:
		return kind
	}
}
