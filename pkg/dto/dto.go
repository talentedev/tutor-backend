package dto

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

var (
	funcTagRegexp = regexp.MustCompile(`(?m)([a-zA-Z_]*)\=([a-zA-Z]*)\((.*)\)`)
)

type Model struct{}

type Context struct {
	Props map[string]interface{}
}

func value(o reflect.Value, field int, tag string, ctx *Context) (prop string, v interface{}) {

	if funcTagRegexp.Match([]byte(tag)) {

		matches := funcTagRegexp.FindStringSubmatch(tag)
		prop = matches[1]
		method := o.MethodByName(matches[2])

		if !method.IsValid() {
			panic(fmt.Sprintf(
				"func(o %s) %s() {} method on value is undefined",
				o.Type().Name(),
				matches[2],
			))
		}

		var params []reflect.Value = make([]reflect.Value, 0)

		if matches[3] != "" {
			for _, v := range strings.Split(matches[3], ",") {
				params = append(params, ctx.Get(v))
			}
		}

		// Verify param len for dto func
		if method.Type().NumIn() != len(params) {
			panic(fmt.Sprintf(
				"Dto func %s for type %s has %d params and provided %d",
				matches[2],
				o.Type().Name(),
				method.Type().NumIn(),
				len(params),
			))
		}

		// Verify param types of dto func
		for i := 0; i < len(params); i++ {
			if method.Type().In(i).Kind() != params[i].Kind() {
				panic(fmt.Sprintf(
					"Dto func %s for type %s mismatch param types, expected %s for param %d and provided %s",
					matches[2],
					o.Type().Name(),
					method.Type().In(i).Kind().String(),
					i,
					params[i].Kind().String(),
				))
			}
		}

		res := method.Call(params)

		if len(res) != 1 {
			panic(fmt.Sprintf(
				"Method '%s' should return only one value",
				matches[2],
			))
		}

		v = res[0].Interface()

		return
	}

	return tag, o.Field(field).Interface()
}

func serialize(o reflect.Value, ctx *Context) reflect.Value {

	r := make(map[string]interface{}, 0)

	for i := 0; i < o.NumField(); i++ {

		f := o.Type().Field(i)
		fn := f.Name
		tag := f.Tag.Get("dto")
		tagIf := f.Tag.Get("dtoif")

		if tag == "" {
			continue
		}

		if tagIf != "" {

			tagIfv, isBool := ctx.Get(tagIf).Interface().(bool)

			if !isBool {
				panic(fmt.Sprintf(
					"Property %s is not bool",
					tagIf,
				))
			}

			if !tagIfv {
				continue
			}

		}

		if fn == "_" {
			key, value := value(o, i, tag, ctx)
			r[key] = value
		}

		if f.Type.Name() == "Model" {
			r["__dto"] = strings.TrimLeft(tag, ":")
			continue
		}

		var hasDto bool

		var fieldType reflect.Type = o.Field(i).Type()
		var fieldValue reflect.Value = o.Field(i)

		if fieldType.Kind() == reflect.Ptr {
			pv := o.Field(i).Interface()
			fieldValue = reflect.Indirect(reflect.ValueOf(pv))
			fieldType = fieldValue.Type()
		}

		if fieldType.Kind() == reflect.Struct {
			_, hasDto = fieldType.FieldByName("Model")
		}

		if hasDto {
			r[tag] = serialize(fieldValue, ctx).Interface()
		} else {
			key, value := value(o, i, tag, ctx)
			r[key] = value
		}
	}

	return reflect.ValueOf(r)
}

// Create json string based on struct tags
func Serialize(o interface{}, params ...*prop) interface{} {
	v := reflect.Indirect(reflect.ValueOf(o))
	ctx := newContext(params...)

	if v.Kind() == reflect.Slice {
		a := make([]interface{}, 0)
		for i := 0; i < v.Len(); i++ {
			a = append(a, serialize(v.Index(i), ctx).Interface())
		}
		return a
	}

	return serialize(v, ctx).Interface()
}

func newContext(props ...*prop) *Context {
	var m map[string]interface{} = make(map[string]interface{}, 0)
	for _, prop := range props {
		m[prop.Name] = prop.Value
	}
	return &Context{Props: m}
}

func (c Context) Get(varName string) reflect.Value {

	if strings.Index(strings.Trim(varName, " "), "$") != 0 {
		panic(fmt.Sprintf(
			"'%s' is not a variable",
			varName,
		))
	}

	varName = strings.Split(varName, "$")[1]

	for key, value := range c.Props {
		if key == varName {

			rv := reflect.ValueOf(value)

			if rv.Kind() == reflect.Func {
				fnResp := rv.Call([]reflect.Value{})
				if len(fnResp) != 1 {
					panic(fmt.Sprintf(
						"Dto context func of %s should return only one result",
						key,
					))
				}
				return fnResp[0]
			}

			return rv
		}
	}

	panic(fmt.Sprintf(
		"Variable %s is undefined in context",
		varName,
	))
}

type prop struct {
	Name  string
	Value interface{}
}

func Prop(name string, value interface{}) *prop {
	return &prop{name, value}
}
