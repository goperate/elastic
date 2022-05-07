package basics

import (
	"context"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/olivere/elastic"
	"reflect"
	"sort"
	"strings"
)

var customSearchGlobal func(string) []elastic.Query
var customSorterGlobal func(string) []elastic.Sorter

type StructToEsQuery struct {
	// {must/not/should/filter/group: map[string]*StructToEsQuery}
	logical map[string]map[string]*StructToEsQuery
	sorters map[int][]elastic.Sorter
	levels  []int

	type_      string // nested, obj, field, logical, group
	innerHits  EsInnerHits
	relational string
	parent     string
	fields     []string
	val        []interface{}
	querys     []elastic.Query
}

func NewStructToEsQuery() *StructToEsQuery {
	return &StructToEsQuery{}
}

func NewStructToEsQueryAndCustomSearch(customSearch func(string) []elastic.Query) *StructToEsQuery {
	customSearchGlobal = customSearch
	return NewStructToEsQuery()
}

func NewStructToEsQueryAndCustomSorter(customSorter func(string) []elastic.Sorter) *StructToEsQuery {
	customSorterGlobal = customSorter
	return NewStructToEsQuery()
}

func NewStructToEsQueryAndCustom(customSearch func(string) []elastic.Query, customSorter func(string) []elastic.Sorter) *StructToEsQuery {
	customSearchGlobal = customSearch
	return NewStructToEsQueryAndCustomSorter(customSorter)
}

func (t *StructToEsQuery) setLogical(logical, key string, val *StructToEsQuery) *StructToEsQuery {
	if t.logical == nil {
		t.logical = make(map[string]map[string]*StructToEsQuery)
	}
	if t.logical[logical] == nil {
		t.logical[logical] = make(map[string]*StructToEsQuery)
	}
	if t.logical[logical][key] != nil {
		return t.logical[logical][key]
	}
	t.logical[logical][key] = val
	return val
}

func (t *StructToEsQuery) getLogical(logical string) map[string]*StructToEsQuery {
	if t.logical == nil {
		return nil
	}
	return t.logical[logical]
}

func (t *StructToEsQuery) getLogicalStruct(logical, key string) *StructToEsQuery {
	if t.getLogical(logical) == nil {
		return nil
	}
	return t.logical[logical][key]
}

func (t *StructToEsQuery) setParent(parent, val string) {
	if parent == "" {
		t.parent = val
		return
	}
	t.parent = parent + "." + val
}

func (t *StructToEsQuery) getNames(fieldName string, tag reflect.StructTag) []string {
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

func (t *StructToEsQuery) getVal(v reflect.Value) []interface{} {
	switch v.Kind() {
	case reflect.Ptr:
		return t.getVal(v.Elem())
	case reflect.Invalid:
		return nil
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

	Sort  string
	Type  string // number/string
	Mode  string
	Level int

	Custom bool
	Block  bool
}

func (t *StructToEsQuery) getTags(tag string) (res *esTags) {
	if tag == "-" {
		return nil
	}
	res = new(esTags)
	res.Logical = []string{"must"}
	res.Type = "string"
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
			case "sort":
				res.Sort = kv[1]
			case "mode":
				res.Mode = kv[1]
			case "level":
				res.Level = jsoniter.WrapString(kv[1]).ToInt()
				if res.Level == 0 && kv[1] != "0" {
					panic("level值只能是整数")
				}
			case "type":
				res.Type = kv[1]
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
		case "sort":
			res.Sort = "default"
		case "custom":
			res.Custom = true
		case "block":
			res.Block = true
		}
	}
	return
}

func (t *StructToEsQuery) analysisLogical(name string, tags *esTags) *StructToEsQuery {
	this := t
	length := len(tags.Logical)
	for j, logical := range tags.Logical {
		ss := strings.Split(logical, "@")
		group := ""
		if len(ss) > 1 {
			if j == length-1 && tags.Nesting != "innerHits" {
				panic("分组不能作为最后一个逻辑运算")
			}
			logical = ss[0]
			group = ss[1]
		}
		next := new(StructToEsQuery)
		next.parent = this.parent
		next.type_ = "logical"
		key := ""
		if j == length-1 {
			key = name
		} else {
			nextLogical := strings.Split(tags.Logical[j+1], "@")
			if nextLogical[0] == "nested" {
				key = nextLogical[1]
				next.type_ = "nested"
				next.setParent(this.parent, key)
			}
		}
		switch logical {
		case "must", "not", "should", "filter":
			this = this.setLogical(logical, key, next)
		case "nested":
			// nested 作为path解析不分组解析
			group = ""
		default:
			panic(logical + ": 仅支持传入 must, not, should, filter")
		}
		if group == "" {
			continue
		}
		if this.getLogicalStruct("group", group) != nil {
			this = this.getLogicalStruct("group", group)
			continue
		}
		next = new(StructToEsQuery)
		next.type_ = "group"
		this = this.setLogical("group", group, next)
	}
	return this
}

func (t *StructToEsQuery) setSorter(fields []string, tags *esTags, val reflect.Value) {
	if tags.Sort == "nested" {
		this := new(StructToEsQuery)
		this.type_ = "nestedSort"
		this.setParent(t.parent, fields[0])
		this.analysis(val)
		querys := this.toQuery()
		if len(querys) == 0 {
			return
		}
		for level, sorters := range this.sorters {
			for _, sorter := range sorters {
				if t.sorters == nil {
					t.sorters = make(map[int][]elastic.Sorter)
					t.levels = append(t.levels, level)
					t.sorters[tags.Level] = make([]elastic.Sorter, 0)
				} else if t.sorters[level] == nil {
					t.levels = append(t.levels, level)
					t.sorters[tags.Level] = make([]elastic.Sorter, 0)
				}
				sorter.(*elastic.FieldSort).Nested(
					elastic.NewNestedSort(this.parent).Filter(querys[0]),
				)
				t.sorters[level] = append(t.sorters[level], sorter)
			}
		}
		return
	}
	if t.sorters == nil {
		t.sorters = make(map[int][]elastic.Sorter)
		t.levels = append(t.levels, tags.Level)
		t.sorters[tags.Level] = make([]elastic.Sorter, 0)
	} else if t.sorters[tags.Level] == nil {
		t.levels = append(t.levels, tags.Level)
		t.sorters[tags.Level] = make([]elastic.Sorter, 0)
	}
	if tags.Custom {
		if customSorterGlobal == nil {
			return
		}
		t.sorters[tags.Level] = append(t.sorters[tags.Level], customSorterGlobal(fields[0])...)
		return
	}
	vv := t.getVal(val)
	if len(vv) == 0 {
		return
	}
	for _, field := range fields {
		if t.type_ == "nestedSort" {
			if tags.Sort != "default" {
				continue
			}
			if t.parent != "" {
				field = t.parent + "." + field
			}
			sorter := elastic.NewFieldSort(field).Order(vv[0] == 2).SortMode(tags.Mode)
			t.sorters[tags.Level] = append(t.sorters[tags.Level], sorter)
			continue
		}
		switch tags.Sort {
		case "default":
			// 2 降序排序, 其它值升序排序
			t.sorters[tags.Level] = append(t.sorters[tags.Level], elastic.SortInfo{Field: field, Ascending: vv[0] == 2})
		case "val":
			// 按照传入的值排序
			m := make(map[string]interface{})
			for index, v := range vv {
				m[jsoniter.Wrap(v).ToString()] = index
			}
			sorter := elastic.NewScriptSort(
				elastic.NewScript(fmt.Sprintf("params.idMap[String.valueOf(doc['%s'].value)]", field)).
					Param("idMap", m),
				tags.Type,
			).Order(true)
			t.sorters[tags.Level] = append(t.sorters[tags.Level], sorter)
		}
	}
}

func (t *StructToEsQuery) analysis(value reflect.Value) {
	if value.Kind() == reflect.Ptr {
		t.analysis(value.Elem())
		return
	}
	if value.Kind() == reflect.Invalid {
		return
	}
	typ := value.Type()
	for i := 0; i < value.NumField(); i++ {
		v := value.Field(i)
		tt := typ.Field(i)
		tags := t.getTags(tt.Tag.Get("es"))
		if tags == nil {
			continue
		}
		if tt.Anonymous || tags.Block { // 匿名字段或分块结构(单纯为了结构分块而添加的嵌套结构)
			if tags.Nesting == "innerHits" {
				if v.Kind() != reflect.Ptr {
					v = v.Addr()
				}
				if !v.IsNil() {
					t.innerHits = v.Interface().(EsInnerHits)
				}
				continue
			}
			t.analysis(v)
			continue
		}
		fields := t.getNames(tt.Name, tt.Tag)
		if tags.Sort != "" {
			t.setSorter(fields, tags, v)
			continue
		}
		this := t.analysisLogical(tt.Name, tags)
		switch tags.Nesting {
		case "nested", "obj":
			this.type_ = tags.Nesting
			this.fields = fields
			this.setParent(t.parent, fields[0])
			this.analysis(v)
		case "innerHits":
			if v.IsNil() {
				continue
			}
			if this.type_ == "nested" {
				this.innerHits = v.Interface().(EsInnerHits)
			} else if t.type_ == "nested" {
				t.innerHits = v.Interface().(EsInnerHits)
			}
		default:
			if tags.Custom {
				if customSearchGlobal == nil {
					continue
				}
				this.type_ = "custom"
				this.querys = append(this.querys, customSearchGlobal(fields[0])...)
				continue
			}
			this.type_ = "val"
			this.relational = tags.Relational
			this.fields = fields
			this.val = this.getVal(v)
		}
	}
	return
}

func (t *StructToEsQuery) valToQuery() (query []elastic.Query) {
	length := len(t.val)
	if length == 0 {
		return
	}
	for _, name := range t.fields {
		if t.parent != "" {
			name = t.parent + "." + name
		}
		switch t.relational {
		case "":
			if length == 1 {
				query = append(query, elastic.NewTermQuery(name, t.val[0]))
			} else {
				query = append(query, elastic.NewTermsQuery(name, t.val...))
			}
		case "match":
			if length == 1 {
				query = append(query, elastic.NewMatchQuery(name, t.val[0]))
				continue
			}
			q := elastic.NewBoolQuery()
			for _, v := range t.val {
				q.Should(elastic.NewMatchQuery(name, v))
			}
			query = append(query, q)
		case "matchAnd":
			if length == 1 {
				query = append(query, elastic.NewMatchQuery(name, t.val[0]).Operator("and"))
				continue
			}
			q := elastic.NewBoolQuery()
			for _, v := range t.val {
				q.Should(elastic.NewMatchQuery(name, v).Operator("and"))
			}
			query = append(query, q)
		case "range":
			if length == 1 {
				query = append(query, elastic.NewRangeQuery(name).Gte(t.val[0]))
			} else if t.val[0] == "" {
				query = append(query, elastic.NewRangeQuery(name).Lte(t.val[1]))
			} else {
				query = append(query, elastic.NewRangeQuery(name).Gte(t.val[0]).Lte(t.val[1]))
			}
		case "rangeLte":
			if length == 1 {
				query = append(query, elastic.NewRangeQuery(name).Lte(t.val[0]))
			} else if t.val[0] == "" {
				query = append(query, elastic.NewRangeQuery(name).Lte(t.val[1]))
			} else {
				query = append(query, elastic.NewRangeQuery(name).Gte(t.val[0]).Lte(t.val[1]))
			}
		case "rangeIgnore0":
			rangeQuery := elastic.NewRangeQuery(name)
			if !reflect.ValueOf(t.val[0]).IsZero() {
				rangeQuery.Gte(t.val[0])
			}
			if length > 1 && !reflect.ValueOf(t.val[1]).IsZero() {
				rangeQuery.Lte(t.val[1])
			}
			query = append(query, rangeQuery)
		case "rangeLteIgnore0":
			if length == 1 {
				if reflect.ValueOf(t.val[0]).IsZero() {
					continue
				}
				query = append(query, elastic.NewRangeQuery(name).Lte(t.val[0]))
				continue
			}
			rangeQuery := elastic.NewRangeQuery(name)
			if !reflect.ValueOf(t.val[0]).IsZero() {
				rangeQuery.Gte(t.val[0])
			}
			if !reflect.ValueOf(t.val[1]).IsZero() {
				rangeQuery.Lte(t.val[1])
			}
			query = append(query, rangeQuery)
		case "lt":
			query = append(query, elastic.NewRangeQuery(name).Lt(t.val[0]))
		case "lte":
			query = append(query, elastic.NewRangeQuery(name).Lte(t.val[0]))
		case "gt":
			query = append(query, elastic.NewRangeQuery(name).Gt(t.val[0]))
		case "gte":
			query = append(query, elastic.NewRangeQuery(name).Gte(t.val[0]))
		default:
			panic("relational: " + t.relational + " 不存在")
		}
	}
	return
}

func (t *StructToEsQuery) mapToQuery(val map[string]*StructToEsQuery) (query []elastic.Query) {
	for _, v := range val {
		if v.type_ == "obj" || v.type_ == "nested" {
			query = append(query, v.toQuery()...)
			continue
		}
		if v.getLogical("group") != nil {
			query = append(query, v.mapToQuery(v.getLogical("group"))...)
			continue
		}
		if v.type_ == "custom" {
			query = append(query, v.querys...)
		} else if v.type_ != "val" {
			query = append(query, v.toQuery()...)
		} else {
			query = append(query, v.valToQuery()...)
		}
	}
	return
}

func (t *StructToEsQuery) toQuery() (query []elastic.Query) {
	if t.logical == nil {
		return
	}
	use := false
	bq := elastic.NewBoolQuery()
	for logical, info := range t.logical {
		if info == nil || logical == "group" {
			continue
		}
		qs := t.mapToQuery(info)
		use = use || len(qs) > 0
		switch logical {
		case "must":
			bq.Must(qs...)
		case "not":
			bq.MustNot(qs...)
		case "should":
			bq.Should(qs...)
		case "filter":
			bq.Filter(qs...)
		}
	}
	if !use {
		return
	}
	if t.type_ != "nested" {
		query = append(query, bq)
		return
	}
	nq := elastic.NewNestedQuery(t.parent, bq)
	if t.innerHits != nil {
		nq.InnerHit(t.innerHits.InitInnerHits())
	}
	query = append(query, nq)
	return
}

func (t *StructToEsQuery) ToQuery(form interface{}) *elastic.BoolQuery {
	t.analysis(reflect.ValueOf(form))
	querys := t.toQuery()
	if len(querys) == 0 {
		return elastic.NewBoolQuery()
	}
	return querys[0].(*elastic.BoolQuery)
}

func (t *StructToEsQuery) GetSorters() (res []elastic.Sorter) {
	sort.Ints(t.levels)
	for _, level := range t.levels {
		res = append(res, t.sorters[level]...)
	}
	return
}

func (t *StructToEsQuery) Search(req *elastic.SearchService, form interface{}) (res *elastic.SearchHits, err error) {
	req.Query(t.ToQuery(form)).SortBy(t.GetSorters()...)
	if t.innerHits != nil {
		t.innerHits.SetSource(req)
	}
	sr, err := req.Do(context.Background())
	if err != nil || sr.Hits.TotalHits == 0 {
		return
	}
	res = sr.Hits
	return
}

func (t *StructToEsQuery) ToSearchBody(form interface{}) *SearchBody {
	res := NewSearchBody(t.ToQuery(form)).SetSorter(t.GetSorters()...)
	if t.innerHits != nil {
		res.SetPage(t.innerHits.GetPage()).SetSize(t.innerHits.GetSize())
		if len(t.innerHits.GetInclude())+len(t.innerHits.GetExclude()) > 0 {
			res.Source = elastic.NewFetchSourceContext(true).
				Include(t.innerHits.GetInclude()...).
				Exclude(t.innerHits.GetExclude()...)
		}
	}
	return res
}

type SearchBody struct {
	Query  *elastic.BoolQuery
	Page   int
	Size   int
	Source *elastic.FetchSourceContext
	Sorter []elastic.Sorter
}

func NewSearchBody(query *elastic.BoolQuery) *SearchBody {
	return &SearchBody{
		Query: query,
		Page:  1,
		Size:  10,
	}
}

func (t *SearchBody) Include(val ...string) *SearchBody {
	if len(val) == 0 {
		return t
	}
	if t.Source == nil {
		t.Source = elastic.NewFetchSourceContext(true)
	}
	t.Source.Include(val...)
	return t
}

func (t *SearchBody) Exclude(val ...string) *SearchBody {
	if len(val) == 0 {
		return t
	}
	if t.Source == nil {
		t.Source = elastic.NewFetchSourceContext(true)
	}
	t.Source.Exclude(val...)
	return t
}

func (t *SearchBody) SetPage(page int) *SearchBody {
	if page > 0 {
		t.Page = page
	}
	return t
}

func (t *SearchBody) SetSize(size int) *SearchBody {
	if size > 10000 {
		t.Size = 10000
	} else if size > 0 {
		t.Size = size
	}
	return t
}

func (t *SearchBody) SetSort(field string, ascending bool) *SearchBody {
	t.Sorter = append(t.Sorter, elastic.SortInfo{Field: field, Ascending: ascending})
	return t
}

func (t *SearchBody) SetSorter(val ...elastic.Sorter) *SearchBody {
	t.Sorter = append(t.Sorter, val...)
	return t
}

func (t *SearchBody) Search(req *elastic.SearchService) (res *elastic.SearchHits, err error) {
	req.Query(t.Query).SortBy(t.Sorter...)
	if t.Source != nil {
		req.FetchSourceContext(t.Source)
	}
	sr, err := req.Do(context.Background())
	if err != nil || sr.Hits.TotalHits == 0 {
		return
	}
	res = sr.Hits
	return
}
