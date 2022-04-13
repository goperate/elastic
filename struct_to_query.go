package es

import (
	"github.com/goperate/es/basics"
	"github.com/olivere/elastic"
	"reflect"
	"strings"
)

type StructToQuery struct {
	group   map[string]*elastic.BoolQuery // 存储同一分组的query
	form    reflect.Value
	exclude []string
}

func NewStructToQuery(form interface{}) *StructToQuery {
	return &StructToQuery{
		group: make(map[string]*elastic.BoolQuery),
		form:  reflect.ValueOf(form),
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

func (stq *StructToQuery) getRelationalQuery(tag, parent string, names []string, val []interface{}) (res []elastic.Query) {
	vl := len(val)
	if vl == 0 {
		return
	}
	for _, name := range names {
		if parent != "" {
			name = parent + "." + name
		}
		switch tag {
		case "range": // 支持 >= 和 范围查询
			if vl == 1 {
				res = append(res, elastic.NewRangeQuery(name).Gte(val[0]))
			} else if val[0] == "" {
				res = append(res, elastic.NewRangeQuery(name).Lte(val[1]))
			} else {
				res = append(res, elastic.NewRangeQuery(name).Gte(val[0]).Lte(val[1]))
			}
		case "match":
			for _, v := range val {
				res = append(res, elastic.NewMatchQuery(name, v))
			}
		case "matchAnd":
			for _, v := range val {
				res = append(res, elastic.NewMatchQuery(name, v).Operator("and"))
			}
		default:
			if vl == 1 {
				res = append(res, elastic.NewTermQuery(name, val[0]))
			} else {
				res = append(res, elastic.NewTermsQuery(name, val...))
			}
		}
	}
	return
}

// parent: 嵌套结构的父key
// names: 查询对应的es字段
// val: 传入的查询的值
func (stq *StructToQuery) getLogicalQuery(tags map[string][]string, parent string, names []string, value reflect.Value, query *elastic.BoolQuery) bool {
	logicals := tags["logical"]
	length := len(logicals)
	var querys []elastic.Query
	field := names[0]
	if parent != "" {
		field = parent + "." + field
	}
	switch tags["nesting"][0] {
	case "nested":
		query2 := elastic.NewBoolQuery()
		if innerHits, use := stq.toBodyQuery(field, value, query2); use {
			nq := elastic.NewNestedQuery(field, query2)
			if innerHits != nil {
				nq.InnerHit(innerHits.InitInnerHits())
				stq.exclude = append(stq.exclude, field)
			}
			querys = append(querys, nq)
		}
	case "obj":
		if length > 0 {
			query2 := elastic.NewBoolQuery()
			if _, use := stq.toBodyQuery(field, value, query2); use {
				querys = append(querys, query2)
			}
		} else {
			_, use := stq.toBodyQuery(field, value, query)
			return use
		}
	default:
		querys = stq.getRelationalQuery(tags["relational"][0], parent, names, stq.getVal(value))
	}
	if len(querys) == 0 {
		return false
	}
	if length == 0 {
		query.Must(querys...)
		return true
	}
	for i, logical := range logicals {
		ss := strings.Split(logical, "@")
		if len(ss) > 1 {
			if i == length-1 {
				panic("分组不能作为最后一个逻辑运算")
			}
			if stq.group[ss[1]] != nil {
				query = stq.group[ss[1]]
			} else {
				stq.group[ss[1]] = query
			}
			logical = ss[0]
		}
		var querys2 []elastic.Query
		oldQuery := query
		if i < length {
			query = elastic.NewBoolQuery()
			querys2 = []elastic.Query{query}
		} else {
			querys2 = querys
		}
		switch logical {
		case "should":
			oldQuery.Should(querys2...)
		case "not":
			oldQuery.MustNot(querys2...)
		case "filter":
			oldQuery.Filter(querys2...)
		case "must":
			oldQuery.Must(querys2...)
		default:
			panic("请指定正确的逻辑运算")
		}
	}
	query2 := elastic.NewBoolQuery()
	for i := length - 1; i >= 0; i-- {
		if i == 0 {
			query2 = query
		}
		logical := logicals[i]
		ss := strings.Split(logical, "@")
		over := false
		if len(ss) > 1 {
			if i == length-1 {
				panic("分组不能作为最后一个逻辑运算")
			}
			if stq.group[ss[1]] != nil {
				query2 = stq.group[ss[1]]
				over = true
			} else {
				stq.group[ss[1]] = querys[0].(*elastic.BoolQuery)
			}
			logical = ss[0]
		}
		switch logical {
		case "should":
			querys = []elastic.Query{query2.Should(querys...)}
		case "not":
			querys = []elastic.Query{query2.MustNot(querys...)}
		case "filter":
			querys = []elastic.Query{query2.Filter(querys...)}
		case "must":
			querys = []elastic.Query{query2.Must(querys...)}
		default:
			panic("请指定正确的逻辑运算")
		}
		if over {
			break
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

func (stq *StructToQuery) getTags(tag string) (res map[string][]string) {
	if tag == "-" {
		return nil
	}
	res = make(map[string][]string)
	for _, v := range strings.Split(tag, ";") {
		kv := strings.Split(v, ":")
		length := len(kv)
		if length == 0 {
			continue
		} else if length >= 2 {
			res[kv[0]] = append(res[kv[0]], strings.Split(kv[1], ",")...)
			continue
		}
		switch v {
		case "nested", "obj", "innerHits":
			res["nesting"] = append(res["nesting"], v)
		case "must", "not", "filter", "should":
			res["logical"] = append(res["logical"], v)
		case "match", "matchAnd", "range", "lt", "lte", "gt", "gte":
			res["relational"] = append(res["relational"], v)
		}
	}
	res["nesting"] = append(res["nesting"], "")
	res["relational"] = append(res["relational"], "")
	return
}

func (stq *StructToQuery) toBodyQuery(field string, value reflect.Value, query *elastic.BoolQuery) (innerHits basics.InnerHits, useQuery bool) {
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if value.Kind() == reflect.Invalid {
		return
	}
	t := value.Type()
	for i := 0; i < value.NumField(); i++ {
		v := value.Field(i)
		tt := t.Field(i)
		if tt.Anonymous {
			innerHits2, useQuery2 := stq.toBodyQuery(field, v, query)
			if innerHits == nil {
				innerHits = innerHits2
			}
			if !useQuery {
				useQuery = useQuery2
			}
			continue
		}
		tags := stq.getTags(tt.Tag.Get("es"))
		if tags == nil {
			continue
		}
		names := stq.getNames(tt.Tag)
		useQuery = stq.getLogicalQuery(tags, field, names, v, query) || useQuery
	}
	return
}

func (stq *StructToQuery) ToBodyQuery() (query *elastic.BoolQuery, exclude []string) {
	query = elastic.NewBoolQuery()
	stq.toBodyQuery("", stq.form, query)
	return
}
