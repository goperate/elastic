package basics

import (
	"fmt"
	"reflect"
	"strings"
)

type StructAnalysis struct {
	must    map[string]*StructAnalysis
	mustNot map[string]*StructAnalysis
	should  map[string]*StructAnalysis
	filter  map[string]*StructAnalysis
	group   map[string]*StructAnalysis

	type_      string // nested, obj, field, logical, group
	innerHits  InnerHits
	relational string
	parent     string
	fields     []string
	val        []interface{}
}

func (t *StructAnalysis) merge(old, val map[string]*StructAnalysis) map[string]*StructAnalysis {
	if old == nil {
		return val
	}
	for k, v := range val {
		old[k] = v
	}
	return old
}

func (t *StructAnalysis) Must(val map[string]*StructAnalysis) {
	t.must = t.merge(t.must, val)
}

func (t *StructAnalysis) MustNot(val map[string]*StructAnalysis) {
	t.mustNot = t.merge(t.mustNot, val)
}

func (t *StructAnalysis) Should(val map[string]*StructAnalysis) {
	t.should = t.merge(t.should, val)
}

func (t *StructAnalysis) Filter(val map[string]*StructAnalysis) {
	t.filter = t.merge(t.filter, val)
}

func (t *StructAnalysis) Group(val map[string]*StructAnalysis) {
	t.group = t.merge(t.group, val)
}

func (t *StructAnalysis) setParent(val string) {
	if t.parent == "" {
		t.parent = val
		return
	}
	t.parent += "." + val
}

func (t *StructAnalysis) getNames(fieldName string, tag reflect.StructTag) []string {
	// name 优先级 esFields > esField > json > fieldName
	esFields := tag.Get("fields")
	if esFields != "" {
		return strings.Split(esFields, ",")
	}
	esField := tag.Get("field")
	if esField != "" {
		return []string{esField}
	}
	json := tag.Get("json")
	if json != "" {
		return []string{json}
	}
	return []string{fieldName}
}

func (t *StructAnalysis) getVal(v reflect.Value) []interface{} {
	if v.IsNil() || v.IsZero() {
		return nil
	}
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return []interface{}{v.Interface()}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return []interface{}{v.Interface()}
	case reflect.Float32, reflect.Float64, reflect.String:
		return []interface{}{v.Interface()}
	case reflect.Slice, reflect.Array:
		vl := v.Len()
		if vl == 0 {
			return nil
		}
		vv := make([]interface{}, vl)
		for j := 0; j < vl; j++ {
			vv[j] = v.Index(j).Interface()
		}
		return vv
	default:
		return nil
	}
}

type esTags struct {
	Nesting    string
	Logical    []string
	Relational string
}

func (t *StructAnalysis) getTags(tag string) (res *esTags) {
	if tag == "-" {
		return nil
	}
	res = new(esTags)
	res.Logical = []string{"must"}
	for _, v := range strings.Split(tag, ";") {
		kv := strings.Split(v, ":")
		length := len(kv)
		if length == 0 {
			continue
		} else if length >= 2 {
			switch kv[0] {
			case "nesting":
				res.Nesting = kv[1]
			case "logical":
				res.Logical = strings.Split(kv[1], ",")
			case "relational":
				res.Relational = kv[1]
			}
			continue
		}
		switch v {
		case "nested", "obj", "innerHits":
			res.Nesting = v
		case "must", "not", "filter", "should":
			res.Logical = []string{v}
		case "match", "matchAnd", "range", "lt", "lte", "gt", "gte":
			res.Relational = v
		}
	}
	return
}

func (t *StructAnalysis) analysisLogical(tags *esTags) *StructAnalysis {
	this := t
	length := len(tags.Logical)
	for j, logical := range tags.Logical {
		obj := make(map[string]*StructAnalysis)
		ss := strings.Split(logical, "@")
		group := ""
		if len(ss) > 1 {
			if j == length-1 {
				panic("分组不能作为最后一个逻辑运算")
			}
			logical = ss[0]
			group = ss[1]
		}
		next := new(StructAnalysis)
		next.type_ = "logical"
		obj[""] = next
		switch logical {
		case "must":
			this.Must(obj)
		case "not":
			this.MustNot(obj)
		case "should":
			this.Should(obj)
		case "filter":
			this.Filter(obj)
		default:
			panic(logical + ": 仅支持传入 must, not, should, filter")
		}
		this = next
		if group == "" {
			continue
		}
		next = new(StructAnalysis)
		next.type_ = "group"
		this.Group(map[string]*StructAnalysis{group: next})
		this = next
	}
	return this
}

func (t *StructAnalysis) analysis(value reflect.Value) {
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if value.Kind() == reflect.Invalid {
		return
	}
	typ := value.Type()
	for i := 0; i < value.NumField(); i++ {
		v := value.Field(i)
		tt := typ.Field(i)
		if tt.Anonymous { //匿名字段
			t.analysis(v)
			continue
		}
		tags := t.getTags(tt.Tag.Get("es"))
		if tags == nil {
			continue
		}
		fields := t.getNames(tt.Name, tt.Tag)
		this := t.analysisLogical(tags)
		switch tags.Nesting {
		case "nested", "obj":
			next := new(StructAnalysis)
			next.type_ = tags.Nesting
			next.fields = fields
			this.Group(map[string]*StructAnalysis{tt.Name: next})
			next.analysis(v)
		case "innerHits":
			t.innerHits = v.Interface().(InnerHits)
		default:
			this.relational = tags.Relational
			this.fields = fields
			this.val = this.getVal(v)
		}
	}
	return
}

func (t *StructAnalysis) Analysis(form interface{}) {
	t.analysis(reflect.ValueOf(form))
	fmt.Println("----------end-----------")
}
