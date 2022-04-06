package main

import (
	"context"
	"fmt"
	"github.com/goperate/es"
	"github.com/goperate/es/test/config"
	jsoniter "github.com/json-iterator/go"
	"github.com/spf13/viper"
)

type FormBase struct {
	Integer es.ArrayInt     `json:"integer"`
	Long    es.ArrayInt64   `json:"long" es:"range"`
	Keyword es.ArrayKeyword `json:"keyword"` // ArrayKeyword 只能用于不包含英文逗号和空白符的字符串
	Date    es.ArrayKeyword `json:"date"`
	Text    es.ArrayString  `json:"text"`
	// 同一分组之间should, 多个字段之间should
	ErpArea       es.ArrayInt `json:"erpArea" es:"should" fields:"goodsErpArea,userErpArea" group:"erpAreas"`
	ErpAreas      es.ArrayInt `json:"erpAreas" es:"should" fields:"goodsErpAreas.area,userErpAreas.area" group:"erpAreas"`
	GoodsErpArea  es.ArrayInt `json:"goodsErpArea" es:"should" group:"goodsErpAreas"`
	GoodsErpAreas es.ArrayInt `json:"goodsErpAreas" es:"should" field:"goodsErpAreas.area" group:"goodsErpAreas"`
	UserErpArea   es.ArrayInt `json:"userErpArea" es:"should" group:"userErpAreas"`
	UserErpAreas  es.ArrayInt `json:"userErpAreas" es:"should" field:"userErpAreas.area" group:"userErpAreas"`
}

type Form struct {
	FormBase
	FormBase2 FormBase
	Nested    *FormBase `json:"nested" es:"nested"`
	Obj       *FormBase `json:"obj" es:"obj"`
}

func (t *Form) Search() {
	query, _ := es.NewStructToQuery(config.TEST_MAPPING["mappings"].(map[string]interface{}), t).ToBodyQuery()
	req := config.Es().Search().Index(viper.GetString("es.index"))
	req.Query(query)
	_, err := req.Do(context.Background())
	fmt.Println(err)
}

func main() {
	form := new(Form)
	json := `{
		"integer": 10,
		"long": 100
	}`
	_ = jsoniter.UnmarshalFromString(json, form)
	fmt.Println(jsoniter.MarshalToString(form))
	form.Search()
}
