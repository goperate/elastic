# 1. 连接es
```yaml
es:
  address: http://127.0.0.1:9200
  username: elastic
  password: test
  index: elastic-test-20220402
```
```go
package conn

import (
	"github.com/olivere/elastic"
	"github.com/spf13/viper"
	"log"
	"os"
	"time"
)

func init() {
	// 读取yaml文件
	// config := viper.New() // 通过New加载配置则只能用其返回值获取配置
	config := viper.GetViper()  // 全局加载配置, 可在任意位置获取配置
	config.AddConfigPath("./")  //设置读取的文件路径
	config.SetConfigName("app") //设置读取的文件名
	config.SetConfigType("yaml")
	if err := config.ReadInConfig(); err != nil {
		panic(err)
	}
}

var con *elastic.Client

func init() {
	// 初始化es连接
	options := []elastic.ClientOptionFunc{
		elastic.SetURL(viper.GetString("es.address")),
		elastic.SetSniff(viper.GetBool("es.sniff")),
		elastic.SetHealthcheckInterval(10 * time.Second),
		elastic.SetGzip(viper.GetBool("es.gzip")),
		elastic.SetErrorLog(log.New(os.Stderr, "ELASTIC ", log.LstdFlags)),
		elastic.SetInfoLog(log.New(os.Stdout, "", log.LstdFlags)),
		elastic.SetBasicAuth(viper.GetString("es.username"), viper.GetString("es.password")),
	}
	var err error
	con, err = elastic.NewClient(options...)
	if err != nil {
		panic(err)
	}
}

func Es() *elastic.Client {
	return con
}
```



# 2. 查询
## 2.1 term/terms
```go
package main

import (
	"app/conn"
	"github.com/goperate/es/basics"
	"github.com/spf13/viper"
)

type TestForm struct {
	Id int `json:"id"`
}

func main() {
	obj := basics.NewStructToEsQuery()
	req := conn.Es().Search().Index(viper.GetString("es.index"))
	form := new(TestForm)
	_, _ = obj.Search(req, form)
}
```
对应的es查询json如下
```json
{"query":{"bool":{"must":{"term":{"id":0}}}}}
```
> 我们发现在未对form赋值的情况下会出现一条id为0的查询, 那么我们如何避免这种情况呢
```go
type TestForm struct {
    Id  *int            `json:"id"`
    Id2 []int           `json:"id2" field:"id"`
    Id3 basics.ArrayInt `json:"id3" field:"id"`
}
```
> 我们使用上述几种类型则不会出现不赋值也被解析的情况

> basics.ArrayInt的优点: 当你需要把json字符串解析为结构体时它可以解析, 字符串, 数字, 数组甚至逗号分割的字符串
> 
> 例如: 1, "1", [1, 2], ["1", "2"], "1,2"
```go
func main() {
	obj := basics.NewStructToEsQuery()
	req := conn.Es().Search().Index(viper.GetString("es.index"))
	form := new(TestForm)
	jsonStr := "{\"id\": 1, \"id2\": [10, 20], \"id3\": 30}"
	_ = json.Unmarshal([]byte(jsonStr), form)
	_, _ = obj.Search(req, form)
}
```
```json
{"query":{"bool":{"must":[{"term":{"id":1}},{"terms":{"id":[10,20]}},{"term":{"id":30}}]}}}
```
> tag: field 指定查询es中对应的字段, 取值优先级: fields > field > json > 字段名


## 2.2 range/lt/lte/gt/gte
```go
type TestForm struct {
	Id  *int            `json:"id"`
	Id2 []int           `json:"id2" field:"id" es:"range"`
	Id3 basics.ArrayInt `json:"id3" field:"id" es:"relational:range"`
}
```
```json
{"query":{"bool":{"must":[{"term":{"id":1}},{"range":{"id":{"from":10,"include_lower":true,"include_upper":true,"to":20}}},{"range":{"id":{"from":30,"include_lower":true,"include_upper":true,"to":null}}}]}}}
```
- range 或 relational:range 当字段值元素数为1时等同于 field >= $val, 当字段值元素数为2时等同于 field >= $val[0] AND field <= $val[1]
- relational:rangeLte 当字段值元素数为1时等同于 field <= $val, 当字段值元素数为2时等同于 field >= $val[0] AND field <= $val[1]
- relational:rangeIgnore0 同range但如果元素值为零值时则忽略, 如[0, 1000]等同于 field <= 1000
- relational:rangeLteIgnore0 同rangeLte但如果元素值为零值时则忽略
- lt 或 relational:lt 同 field < $val
- lte 或 relational:lte 同 field <= $val
- gt 或 relational:gt 同 field > $val
- gte 或 relational:gte 同 field >= $val


## 2.3 match
```go
type TestForm struct {
	Name  *string            `json:"name" es:"relational:match"`
	Name2 basics.ArrayString `json:"name2" field:"name" es:"relational:matchAnd"`
}
```
```json
{"query":{"bool":{"must":[{"match":{"name":{"query":"测试"}}},{"match":{"name":{"operator":"and","query":"测试"}}}]}}}
```
