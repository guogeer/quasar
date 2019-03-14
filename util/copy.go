package util

import (
	"reflect"
)

// 深拷贝
// 结构体之间递归深拷贝
// 整数、浮点数、字符串、布尔类型直接拷贝，其他类型忽略
func DeepCopy(dst, src interface{}) {
	if dst == nil || src == nil {
		return
	}
	sval := reflect.Indirect(reflect.ValueOf(src))
	dval := reflect.Indirect(reflect.ValueOf(dst))
	if !(sval.Kind() == reflect.Struct && dval.Kind() == reflect.Struct) {
		return
	}
	for i := 0; i < sval.NumField(); i++ {
		sfield := sval.Field(i)
		sname := sval.Type().Field(i).Name
		dfield := dval.FieldByName(sname)
		sfield = reflect.Indirect(sfield)
		dfield = reflect.Indirect(dfield)
		if !(sfield.IsValid() && dfield.IsValid()) {
			continue
		}
		if dfield.CanSet() == false {
			continue
		}
		if testKind(sfield.Kind()) != testKind(dfield.Kind()) {
			continue
		}
		switch testKind(sfield.Kind()) {
		case reflect.Int64:
			dfield.SetInt(sfield.Int())
		case reflect.Uint64:
			dfield.SetUint(sfield.Uint())
		case reflect.Float64:
			dfield.SetFloat(sfield.Float())
		case reflect.Bool, reflect.String:
			dfield.Set(sfield)
		case reflect.Struct:
			dfield = dfield.Addr()
			DeepCopy(dfield.Interface(), sfield.Interface())
		}
	}
}

func testKind(k reflect.Kind) reflect.Kind {
	switch k {
	case reflect.Float32, reflect.Float64:
		return reflect.Float64
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Int:
		return reflect.Int64
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Uint:
		return reflect.Uint64
	}
	return k
}
