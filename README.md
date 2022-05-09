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
{
  "query": {
    "bool": {
      "must": {
        "term": {
          "id": 0
        }
      }
    }
  }
}
```

> 我们发现在未对form赋值的情况下会出现一条id为0的查询, 那么我们如何避免这种情况呢

```go
package main

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
package main

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
{
  "query": {
    "bool": {
      "must": [
        {
          "term": {
            "id": 1
          }
        },
        {
          "terms": {
            "id": [
              10,
              20
            ]
          }
        },
        {
          "term": {
            "id": 30
          }
        }
      ]
    }
  }
}
```

> tag: field 指定查询es中对应的字段, 取值优先级: fields > field > json > 字段名

## 2.2 range/lt/lte/gt/gte

```go
package main

type TestForm struct {
	Id  *int            `json:"id"`
	Id2 []int           `json:"id2" field:"id" es:"range"`
	Id3 basics.ArrayInt `json:"id3" field:"id" es:"relational:range"`
}
```

```json
{
  "query": {
    "bool": {
      "must": [
        {
          "term": {
            "id": 1
          }
        },
        {
          "range": {
            "id": {
              "from": 10,
              "include_lower": true,
              "include_upper": true,
              "to": 20
            }
          }
        },
        {
          "range": {
            "id": {
              "from": 30,
              "include_lower": true,
              "include_upper": true,
              "to": null
            }
          }
        }
      ]
    }
  }
}
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
package main

type TestForm struct {
	Name  *string            `json:"name" es:"relational:match"`
	Name2 basics.ArrayString `json:"name2" field:"name" es:"relational:matchAnd"`
}
```

```json
{
  "query": {
    "bool": {
      "must": [
        {
          "match": {
            "name": {
              "query": "测试"
            }
          }
        },
        {
          "match": {
            "name": {
              "operator": "and",
              "query": "测试"
            }
          }
        }
      ]
    }
  }
}
```

## 2.4 must/must_not/should/filter

### 2.4.1 must

```go
package main

type TestForm struct {
	Id   *int    `json:"id"`
	Name *string `json:"name" es:"relational:match"`
}

// 同
type TestForm struct {
	Id   *int    `json:"id" es:"must"`
	Name *string `json:"name" es:"must;relational:match"`
}

// 同
type TestForm struct {
	Id   *int    `json:"id" es:"logical:must"`
	Name *string `json:"name" es:"logical:must;relational:match"`
}
```

```json
{
  "query": {
    "bool": {
      "must": [
        {
          "term": {
            "id": 100
          }
        },
        {
          "match": {
            "name": {
              "query": "测试"
            }
          }
        }
      ]
    }
  }
}
```

> 多个es tag使用 ; 分割, 只有must时可忽略不写

### 2.4.2 must_not

```go
package main

type TestForm struct {
	Id   *int    `json:"id" es:"not"`
	Name *string `json:"name" es:"logical:not;relational:match"`
}
```

```json
{
  "query": {
    "bool": {
      "must_not": [
        {
          "term": {
            "id": 100
          }
        },
        {
          "match": {
            "name": {
              "query": "测试"
            }
          }
        }
      ]
    }
  }
}
```

### 2.4.3 should

```go
package main

type TestForm struct {
	Id   *int    `json:"id" es:"should"`
	Name *string `json:"name" es:"logical:should;relational:match"`
}
```

```json
{
  "query": {
    "bool": {
      "should": [
        {
          "term": {
            "id": 100
          }
        },
        {
          "match": {
            "name": {
              "query": "测试"
            }
          }
        }
      ]
    }
  }
}
```

### 2.4.4 filter

```go
package main

type TestForm struct {
	Id   *int    `json:"id" es:"filter"`
	Name *string `json:"name" es:"logical:filter;relational:match"`
}
```

```json
{
  "query": {
    "bool": {
      "filter": [
        {
          "term": {
            "id": 100
          }
        },
        {
          "match": {
            "name": {
              "query": "测试"
            }
          }
        }
      ]
    }
  }
}
```

### 2.4.5 组合使用

```go
package main

type TestForm struct {
	Id   *int    `json:"id" es:"must"`
	Id2  *int    `json:"id2" es:"filter"`
	Id3  *int    `json:"id3" es:"not"`
	Name *string `json:"name" es:"should;relational:match"`
}
```

```json
{
  "query": {
    "bool": {
      "filter": {
        "term": {
          "id2": 200
        }
      },
      "must": {
        "term": {
          "id": 100
        }
      },
      "must_not": {
        "term": {
          "id3": 300
        }
      },
      "should": {
        "match": {
          "name": {
            "query": "测试"
          }
        }
      }
    }
  }
}
```

### 2.4.6 嵌套使用

#### 2.4.6.1 简单嵌套

```go
package main

type TestForm struct {
	Id  *int `json:"id" es:"must"`
	Id2 *int `json:"id2" es:"logical:must,should"`
	Id3 *int `json:"id3" es:"logical:must,should"`
	Id4 *int `json:"id4" es:"logical:must,not"`
	Id5 *int `json:"id5" es:"logical:must,not"`
}
```

```json
{
  "query": {
    "bool": {
      "must": [
        {
          "term": {
            "id": 100
          }
        },
        {
          "bool": {
            "must_not": [
              {
                "term": {
                  "id4": 400
                }
              },
              {
                "term": {
                  "id5": 500
                }
              }
            ],
            "should": [
              {
                "term": {
                  "id2": 200
                }
              },
              {
                "term": {
                  "id3": 300
                }
              }
            ]
          }
        }
      ]
    }
  }
}
```

> 嵌套时首层的must不能省略, logical不能省略

#### 2.4.6.2 嵌套隔离

```go
package main

type TestForm struct {
	Id  *int `json:"id" es:"must"`
	Id2 *int `json:"id2" es:"logical:must@a,should"`
	Id3 *int `json:"id3" es:"logical:must@a,should"`
	Id4 *int `json:"id4" es:"logical:must@b,not"`
	Id5 *int `json:"id5" es:"logical:must@b,not"`
}
```

```json
{
  "query": {
    "bool": {
      "must": [
        {
          "term": {
            "id": 100
          }
        },
        {
          "bool": {
            "should": [
              {
                "term": {
                  "id2": 200
                }
              },
              {
                "term": {
                  "id3": 300
                }
              }
            ]
          }
        },
        {
          "bool": {
            "must_not": [
              {
                "term": {
                  "id5": 500
                }
              },
              {
                "term": {
                  "id4": 400
                }
              }
            ]
          }
        }
      ]
    }
  }
}
```

> 同一组的会做为一个整体

## 2.5 Object

```go
package main

type TestForm struct {
	Id     *int `json:"id" es:"must" field:"object.id"`
	Id2    *int `json:"id2" es:"must"`
	Object struct {
		Id3 *int `json:"id3" es:"must"`
		Id4 *int `json:"id4" es:"must"`
		Id5 *int `json:"id5" es:"must"`
	} `json:"object" es:"obj"`
}

// 或
type Object struct {
	Id3 *int `json:"id3" es:"must"`
	Id4 *int `json:"id4" es:"must"`
	Id5 *int `json:"id5" es:"must"`
}

type TestForm struct {
	Id     *int    `json:"id" es:"must" field:"object.id"`
	Id2    *int    `json:"id2" es:"must"`
	Object *Object `json:"object" es:"obj"`
}
```

```json
{
  "query": {
    "bool": {
      "must": [
        {
          "term": {
            "object.id": 100
          }
        },
        {
          "term": {
            "id2": 200
          }
        },
        {
          "bool": {
            "must": [
              {
                "term": {
                  "object.id3": 300
                }
              },
              {
                "term": {
                  "object.id4": 400
                }
              },
              {
                "term": {
                  "object.id5": 500
                }
              }
            ]
          }
        }
      ]
    }
  }
}
```

## 2.6 Nested

### 2.6.1 简单使用

```go
package main

type Nested struct {
	Id3 *int `json:"id3" es:"must"`
	Id4 *int `json:"id4" es:"must"`
	Id5 *int `json:"id5" es:"must"`
}

type TestForm struct {
	Id         *int    `json:"id" es:"logical:must,nested@nested_data,must" field:"id"`
	Id2        *int    `json:"id2" es:"must"`
	NestedData *Nested `json:"nested_data" es:"nested"`
}
```

```json
{
  "query": {
    "bool": {
      "must": [
        {
          "nested": {
            "path": "nested_data",
            "query": {
              "bool": {
                "must": {
                  "term": {
                    "nested_data.id": 100
                  }
                }
              }
            }
          }
        },
        {
          "term": {
            "id2": 200
          }
        },
        {
          "nested": {
            "path": "nested_data",
            "query": {
              "bool": {
                "must": [
                  {
                    "term": {
                      "nested_data.id3": 300
                    }
                  },
                  {
                    "term": {
                      "nested_data.id4": 400
                    }
                  },
                  {
                    "term": {
                      "nested_data.id5": 500
                    }
                  }
                ]
              }
            }
          }
        }
      ]
    }
  }
}
```

> 当前版本logical中的nested无法跟nested结构体合并

### 2.6.2 innerHits

```go
package main

import (
	"app/conn"
	"encoding/json"
	"github.com/goperate/es/basics"
	"github.com/spf13/viper"
)

type Nested struct {
	Id3   *int             `json:"id3" es:"must"`
	Id4   *int             `json:"id4" es:"must"`
	Id5   *int             `json:"id5" es:"must"`
	Inner *basics.EsSelect `json:"inner" es:"nesting:innerHits"`
}

type TestForm struct {
	Id               *int             `json:"id" es:"logical:must,nested@nested_data2,must" field:"id"`
	NestedData2Inner *basics.EsSelect `json:"nested_data2_inner" es:"logical:must,nested@nested_data2;nesting:innerHits"`
	Id2              *int             `json:"id2" es:"must"`
	NestedData       *Nested          `json:"nested_data" es:"nested"`
}

func main() {
	obj := basics.NewStructToEsQuery()
	req := conn.Es().Search().Index(viper.GetString("es.index"))
	form := new(TestForm)
	jsonStr := "{\"id\": 100, \"nested_data2_inner\": {}, \"id2\": 200, \"nested_data\": {\"id3\": 300, \"id4\": 400, \"id5\": 500, \"inner\": {\"page\": 2, \"size\": 20, \"include\": [\"aaa\"], \"exclude\": [\"bbb\"]}}}"
	_ = json.Unmarshal([]byte(jsonStr), form)
	_, _ = obj.Search(req, form)
}
```

```json
{
  "query": {
    "bool": {
      "must": [
        {
          "nested": {
            "inner_hits": {
              "from": 0,
              "size": 100
            },
            "path": "nested_data2",
            "query": {
              "bool": {
                "must": {
                  "term": {
                    "nested_data2.id": 100
                  }
                }
              }
            }
          }
        },
        {
          "term": {
            "id2": 200
          }
        },
        {
          "nested": {
            "inner_hits": {
              "_source": {
                "excludes": [
                  "bbb"
                ],
                "includes": [
                  "aaa"
                ]
              },
              "from": 20,
              "size": 20
            },
            "path": "nested_data",
            "query": {
              "bool": {
                "must": [
                  {
                    "term": {
                      "nested_data.id3": 300
                    }
                  },
                  {
                    "term": {
                      "nested_data.id4": 400
                    }
                  },
                  {
                    "term": {
                      "nested_data.id5": 500
                    }
                  }
                ]
              }
            }
          }
        }
      ]
    }
  }
}
```

## 2.7 fields

```go
package main

type TestForm struct {
	Id *int `json:"id" es:"should" fields:"id,id2"`
}
```

```json
{
  "query": {
    "bool": {
      "should": [
        {
          "term": {
            "id": 100
          }
        },
        {
          "term": {
            "id2": 100
          }
        }
      ]
    }
  }
}
```

# 3. 排序

## 3.1 简单排序

```go
package main

import (
	"app/conn"
	"encoding/json"
	"github.com/goperate/es/basics"
	"github.com/spf13/viper"
)

type TestForm struct {
	Id     *int `json:"id"`
	IdSort *int `json:"id_sort" es:"sort" field:"id"`
}

func main() {
	obj := basics.NewStructToEsQuery()
	req := conn.Es().Search().Index(viper.GetString("es.index"))
	form := new(TestForm)
	jsonStr := "{\"id\": 100, \"id_sort\": 2}"
	_ = json.Unmarshal([]byte(jsonStr), form)
	_, _ = obj.Search(req, form)
}
```

```json
{
  "query": {
    "bool": {
      "must": {
        "term": {
          "id": 100
        }
      }
    }
  },
  "sort": [
    {
      "id": {
        "order": "asc"
      }
    }
  ]
}
```

> sort字段的值为2时表示升序排序, 其它任何值为降序排序(建议使用1, 后续升级可能固定为1)

## 3.2 按传入的值排序

```go
package main

import (
	"app/conn"
	"encoding/json"
	"github.com/goperate/es/basics"
	"github.com/spf13/viper"
)

type TestForm struct {
	Id     []int `json:"id"`
	IdSort []int `json:"id_sort" es:"sort:val" field:"id"`
}

func main() {
	obj := basics.NewStructToEsQuery()
	req := conn.Es().Search().Index(viper.GetString("es.index"))
	form := new(TestForm)
	jsonStr := "{\"id\": [100, 200], \"id_sort\": [100, 200]}"
	_ = json.Unmarshal([]byte(jsonStr), form)
	_, _ = obj.Search(req, form)
}
```

```json
{
  "query": {
    "bool": {
      "must": {
        "terms": {
          "id": [
            100,
            200
          ]
        }
      }
    }
  },
  "sort": [
    {
      "_script": {
        "order": "asc",
        "script": {
          "params": {
            "idMap": {
              "100": 0,
              "200": 1
            }
          },
          "source": "params.idMap[String.valueOf(doc['id'].value)]"
        },
        "type": "string"
      }
    }
  ]
}
```

## 3.3 nested

```go
package main

import (
	"app/conn"
	"encoding/json"
	"github.com/goperate/es/basics"
	"github.com/spf13/viper"
)

type TestForm struct {
	Id         []int `json:"id"`
	NestedSort struct {
		StatusSort basics.ArrayInt `json:"status_sort" es:"sort;mode:min" field:"status"`
		Status     basics.ArrayInt `json:"status"`
	} `json:"nested_sort" es:"sort:nested"`
}

func main() {
	obj := basics.NewStructToEsQuery()
	req := conn.Es().Search().Index(viper.GetString("es.index"))
	form := new(TestForm)
	jsonStr := "{\"id\": [100, 200], \"nested_sort\": {\"status\": 6, \"status_sort\": 2}}"
	_ = json.Unmarshal([]byte(jsonStr), form)
	_, _ = obj.Search(req, form)
}
```

```json
{
  "query": {
    "bool": {
      "must": {
        "terms": {
          "id": [
            100,
            200
          ]
        }
      }
    }
  },
  "sort": [
    {
      "nested_sort.status": {
        "mode": "min",
        "nested": {
          "filter": {
            "bool": {
              "must": {
                "term": {
                  "nested_sort.status": 6
                }
              }
            }
          },
          "path": "nested_sort"
        },
        "order": "asc"
      }
    }
  ]
}
```

## 3.4 level控制多字段排序

```go
package main

import (
	"app/conn"
	"encoding/json"
	"github.com/goperate/es/basics"
	"github.com/spf13/viper"
)

type TestForm struct {
	Id         *int  `json:"id"`
	IdSort     []int `json:"id_sort" es:"sort;level:2" field:"id"`
	StatusSort *int  `json:"status_sort" es:"sort;level:1" field:"status"`
}

func main() {
	obj := basics.NewStructToEsQuery()
	req := conn.Es().Search().Index(viper.GetString("es.index"))
	form := new(TestForm)
	jsonStr := "{\"id\": 100, \"id_sort\": [2], \"status_sort\": 2}"
	_ = json.Unmarshal([]byte(jsonStr), form)
	_, _ = obj.Search(req, form)
}
```

```json
{
  "query": {
    "bool": {
      "must": {
        "term": {
          "id": 100
        }
      }
    }
  },
  "sort": [
    {
      "status": {
        "order": "asc"
      }
    },
    {
      "id": {
        "order": "asc"
      }
    }
  ]
}
```
> 多字段排序时可使用level指定字段顺序

# 4. page/size/source

```go
package main

import (
	"app/conn"
	"encoding/json"
	"github.com/goperate/es/basics"
	"github.com/spf13/viper"
)

type TestForm struct {
	Id              []int `json:"id"`
	basics.EsSelect `es:"innerHits"`
}

func main() {
	obj := basics.NewStructToEsQuery()
	req := conn.Es().Search().Index(viper.GetString("es.index"))
	form := new(TestForm)
	jsonStr := "{\"id\": [100, 200], \"page\": 2, \"size\": 20, \"include\": [\"aaa\"], \"exclude\": [\"bbb\"]}"
	_ = json.Unmarshal([]byte(jsonStr), form)
	_, _ = obj.Search(req, form)
}
```

```json
{
  "_source": {
    "excludes": [
      "bbb"
    ],
    "includes": [
      "aaa"
    ]
  },
  "from": 20,
  "query": {
    "bool": {
      "must": {
        "terms": {
          "id": [
            100,
            200
          ]
        }
      }
    }
  },
  "size": 20
}
```

# 5. 忽略层级/忽略字段

```go
package main

import (
	"app/conn"
	"encoding/json"
	"github.com/goperate/es/basics"
	"github.com/spf13/viper"
)

type TestForm struct {
	Id    []int `json:"id"`
	Id2   *int  `json:"id2" es:"-"`
	Other struct {
		Id3 *int `json:"id3"`
	} `json:"other" es:"block"`
}

// 查询效果同
type TestForm struct {
	Id  []int `json:"id"`
	Id3 *int  `json:"id3"`
}

func main() {
	obj := basics.NewStructToEsQuery()
	req := conn.Es().Search().Index(viper.GetString("es.index"))
	form := new(TestForm)
	jsonStr := "{\"id\": [100, 200], \"id2\": 200, \"other\": {\"id3\": 300}}"
	_ = json.Unmarshal([]byte(jsonStr), form)
	_, _ = obj.Search(req, form)
}
```

```json
{
  "query": {
    "bool": {
      "must": [
        {
          "terms": {
            "id": [
              100,
              200
            ]
          }
        },
        {
          "term": {
            "id3": 300
          }
        }
      ]
    }
  }
}
```

> block 表示忽略当前层级, 如示例中的id3跟放在外层对应的查询是一样的

# 6. 自定义

> logical 依然生效

```go
package main

import (
	"app/conn"
	"encoding/json"
	"github.com/goperate/es/basics"
	"github.com/olivere/elastic"
	"github.com/spf13/viper"
)

type TestForm struct {
	Id   *int `json:"id" es:"custom;logical:not"`
	Sort int  `json:"sort" es:"custom;sort"`
}

func (t *TestForm) CustomSearch(field string) []elastic.Query {
	switch field {
	case "id":
		return []elastic.Query{elastic.NewTermsQuery("id2", t.Id)}
	}
	return nil
}

func (t *TestForm) CustomSorter(field string) []elastic.Sorter {
	switch field {
	case "sort":
		return []elastic.Sorter{elastic.SortInfo{Field: "id2", Ascending: t.Sort != 0}}
	}
	return nil
}

func main() {
	form := new(TestForm)
	jsonStr := "{\"id\": 100, \"sort\": 1}"
	_ = json.Unmarshal([]byte(jsonStr), form)
	obj := basics.NewStructToEsQueryAndCustom(form.CustomSearch, form.CustomSorter)
	req := conn.Es().Search().Index(viper.GetString("es.index"))
	_, _ = obj.Search(req, form)
}
```

```json
{
  "query": {
    "bool": {
      "must_not": {
        "terms": {
          "id2": [
            100
          ]
        }
      }
    }
  },
  "sort": [
    {
      "id2": {
        "order": "asc"
      }
    }
  ]
}
```

# 7. 直接修改body

```go
package main

import (
	"app/conn"
	"encoding/json"
	"github.com/goperate/es/basics"
	"github.com/spf13/viper"
)

type TestForm struct {
	Id *int `json:"id" es:"logical:not"`
}

func main() {
	form := new(TestForm)
	jsonStr := "{\"id\": 100}"
	_ = json.Unmarshal([]byte(jsonStr), form)
	obj := basics.NewStructToEsQuery()
	body := obj.ToSearchBody(form)
	body.Query.Must()
	body.Source = nil
	body.SetPage(1).SetSize(20).SetSorter()
	req := conn.Es().Search().Index(viper.GetString("es.index"))
	_, _ = body.Search(req)
}
```
