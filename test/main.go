package main

import (
	"context"
	"fmt"
	"github.com/goperate/es/basics"
	"github.com/goperate/es/test/conf"
	jsoniter "github.com/json-iterator/go"
	"github.com/olivere/elastic"
	"github.com/spf13/viper"
)

type FormBase struct {
	Integer    basics.ArrayInt     `json:"integer"`
	IntegerNot []int               `json:"integerNot" es:"not" field:"integer"`
	Long       basics.ArrayInt64   `json:"long" es:"relational:range"`
	Keyword    basics.ArrayKeyword `json:"keyword"` // ArrayKeyword 只能用于不包含英文逗号和空白符的字符串
	Date       basics.ArrayKeyword `json:"date"`
	Text       basics.ArrayString  `json:"text" fields:"name1,name2"`
	// 同一分组之间should, 多个字段之间should
	Area  basics.ArrayInt `json:"area" es:"logical:must@erpAreas,should" fields:"goodsArea,userArea"`
	Areas basics.ArrayInt `json:"areas" es:"logical:must@erpAreas,should" fields:"goodsAreas.area,userAreas.area"`
	//GoodsArea  es.ArrayInt `json:"goodsArea" es:"logical:must@goodsAreas,should"`
	//GoodsAreas es.ArrayInt `json:"goodsAreas" es:"logical:must@goodsAreas,should" field:"goodsAreas.area"`
	//UserArea   es.ArrayInt `json:"userArea" es:"logical:must@userAreas,should"`
	//UserAreas  es.ArrayInt `json:"userAreas" es:"logical:must@userAreas,should" field:"userAreas.area"`
}

type Form struct {
	FormBase
	Custom *int      `json:"custom" es:"custom"`
	Nested *FormBase `json:"nested" es:"nesting:nested"`
	Obj    *FormBase `json:"obj" es:"obj"`
	Other  struct {
		Id      *int            `json:"id" es:"sort;level:1"`
		Integer basics.ArrayInt `json:"integer" es:"sort:val;level:2"`
		Nested  struct {
			LongSort basics.ArrayInt `json:"longSort" es:"sort;mode:min" field:"long"`
			Integer  basics.ArrayInt `json:"integer"`
		} `json:"nested" es:"sort:nested"`
	} `es:"block"`
}

func (t *Form) Search() {
	obj := basics.NewStructToEsQueryAndCustomSearch(func(s string) []elastic.Query {
		fmt.Println(s, "++++++++++++++++++++")
		switch s {
		case "custom":
			return []elastic.Query{elastic.NewTermsQuery("long", 1, 2, 3, 4)}
		}
		fmt.Println(s, "++++++++++++++++++++")
		return nil
	})
	req := conf.Es().Search().Index(viper.GetString("es.index"))
	req.Query(obj.ToQuery(t)).SortBy(obj.GetSorters()...)
	_, err := req.Do(context.Background())
	fmt.Println(err)
}

func main() {
	form := new(Form)
	json := `{
		"other": {
			"nested": {
				"longSort": 2,
				"Integer": 1000
			}
		},
		"custom": 1,
		"integer": 10,
		"integerNot": [12],
		"long": 100,
		"keyword": ["dqwdqw", "wqdeewd"],
		"date": "2022-04-03 12:00:00",
		"text": "模糊搜索",
		"area": [1, 2],
		"areas": [3, 4],
		"goodsArea": [5, 6],
		"goodsAreas": [7, 8],
		"userAreas": 9,
		"nested": {
			"integer": [1000, 2000],
			"integerNot": [120, 130],
			"long": [3000, 4000],
			"keyword": "aaaaaaaaaaaa",
			"date": ["2022-04-03 12:00:00", "2022-04-04 12:00:00"],
			"area": [10, 20],
			"areas": [30, 40],
			"goodsArea": [50, 60],
			"goodsAreas": [70, 80],
			"userArea": 90
		},
		"obj": {
			"integer": [1001, 2002],
			"long": [3003, 4004],
			"keyword": "xxxxxxxxxxxxxx",
			"date": ["", "2022-04-04 12:00:00"],
			"areas": [300, 400],
			"goodsArea": [500, 600],
			"userAreas": 900
		}
	}`
	_ = jsoniter.UnmarshalFromString(json, form)
	fmt.Println(jsoniter.MarshalToString(form))
	form.Search()
	any := jsoniter.Get([]byte(json))
	fmt.Println(any.Get("obj", "long").ToString(), "============")
}
