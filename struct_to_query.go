package es

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/olivere/elastic"
	"reflect"
	"strings"
)

type StructToQuery struct {
	group   map[string]*elastic.BoolQuery // 存储同一分组的query
	mapping jsoniter.Any
	form    reflect.Value
}

func NewStructToQuery(mapping map[string]interface{}, form interface{}) *StructToQuery {
	mappingString := jsoniter.Wrap(mapping).Get("_doc").Get("properties").ToString()
	return &StructToQuery{
		group:   make(map[string]*elastic.BoolQuery),
		mapping: jsoniter.Get([]byte(mappingString)),
		form:    reflect.ValueOf(form),
	}
}

func (stq *StructToQuery) getNames(tag reflect.StructTag) []string {
	// name 优先级 esFields > esField > json
	names := []string{tag.Get("json")}
	esField := tag.Get("field")
	esFields := tag.Get("fields")
	if esFields != "" {
		names = strings.Split(esFields, ",")
	} else if esField != "" {
		names = []string{esField}
	}
	return names
}

func (stq *StructToQuery) toQuery(tag, name, type_ string, val []interface{}) (res []elastic.Query) {
	vl := len(val)
	if tag == "range" {
		if vl == 1 {
			res = append(res, elastic.NewRangeQuery(name).Gte(val[0]))
		} else {
			res = append(res, elastic.NewRangeQuery(name).Gte(val[0]).Lte(val[1]))
		}
		return
	}
	switch type_ {
	case "text":
		for _, v := range val {
			res = append(res, elastic.NewMatchQuery(name, v).Operator("and"))
		}
	case "date":
		if vl == 1 {
			res = append(res, elastic.NewRangeQuery(name).Gte(val[0]))
		} else if val[0] == "" {
			res = append(res, elastic.NewRangeQuery(name).Lte(val[1]))
		} else {
			res = append(res, elastic.NewRangeQuery(name).Gte(val[0]).Lte(val[1]))
		}
	default:
		res = append(res, elastic.NewTermsQuery(name, val...))
	}
	return
}

// parent: 嵌套结构的父key
// names: 查询对应的es字段
// types: jsoniter.Wrap(es_mapping.ORDER).Get("mappings").Get("_doc").Get("properties"), 用于获取字段在es中的存储类型
// val: 传入的查询的值
func (stq *StructToQuery) setQuery(tag, parent string, names []string, types jsoniter.Any, val []interface{}, query *elastic.BoolQuery) bool {
	if len(val) == 0 {
		return false
	}
	if parent != "" {
		parent = parent + "."
	}
	for _, name := range names {
		type_ := types.Get(name).Get("type").ToString()
		switch tag {
		case "should":
			query.Should(stq.toQuery(tag, parent+name, type_, val)...)
		case "not":
			query.MustNot(stq.toQuery(tag, parent+name, type_, val)...)
		default:
			query.Must(stq.toQuery(tag, parent+name, type_, val)...)
		}
	}
	return true
}

func (stq *StructToQuery) getVal(v reflect.Value) []interface{} {
	if v.Kind() != reflect.Slice {
		return nil
	}
	vl := v.Len()
	if v.IsNil() || vl == 0 {
		return nil
	}
	vv := make([]interface{}, vl)
	for j := 0; j < vl; j++ {
		vv[j] = v.Index(j).Interface()
	}
	return vv
}

func (stq *StructToQuery) toBodyQuery(tag, field string, types jsoniter.Any, value reflect.Value, query *elastic.BoolQuery) (exclude []string) {
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if value.Kind() == reflect.Invalid {
		return
	}
	t := value.Type()
	useQuery := false
	query2 := query
	if tag == "nested" {
		query2 = elastic.NewBoolQuery()
	}
	var innerHits InnerHits
	for i := 0; i < value.NumField(); i++ {
		v := value.Field(i)
		tt := t.Field(i)
		if tt.Anonymous {
			exclude = append(exclude, stq.toBodyQuery(tag, field, types, v, query)...)
			continue
		}
		estag := tt.Tag.Get("es")
		group := tt.Tag.Get("group")
		names := stq.getNames(tt.Tag)
		if estag == "nested" || estag == "obj" {
			name := names[0]
			if field != "" {
				name = field + "." + name
			}
			types2 := types.Get(name).Get("properties")
			exclude = append(exclude, stq.toBodyQuery(estag, name, types2, v, query2)...)
			continue
		}
		if estag == "innerHits" && v.Elem().Kind() != reflect.Invalid {
			innerHits = v.Interface().(InnerHits)
			continue
		}
		vv := stq.getVal(v)
		if group == "" {
			// useQuery 任意一个为true则结果为true
			useQuery = stq.setQuery(estag, field, names, types, vv, query2) || useQuery
			continue
		}
		q2 := elastic.NewBoolQuery()
		if stq.group[group] != nil {
			q2 = stq.group[group]
		}
		if stq.setQuery(estag, field, names, types, vv, q2) {
			stq.group[group] = q2
		}
	}
	for k, q := range stq.group {
		delete(stq.group, k)
		query2.Must(q)
		useQuery = true
	}
	if !useQuery || tag != "nested" {
		return
	}
	nq := elastic.NewNestedQuery(field, query2)
	if innerHits != nil {
		nq.InnerHit(innerHits.InitInnerHits())
		exclude = append(exclude, field)
	}
	query.Filter(nq)
	return
}

func (stq *StructToQuery) ToBodyQuery() (query *elastic.BoolQuery, exclude []string) {
	query = elastic.NewBoolQuery()
	exclude = stq.toBodyQuery("", "", stq.mapping, stq.form, query)
	return
}
