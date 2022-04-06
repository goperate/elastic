package config

type mi = map[string]interface{}

var TEST_MAPPING = mi{
	"settings": mi{
		"number_of_shards":   3,
		"number_of_replicas": 1,
	},
	"mappings": mi{
		"_doc": mi{
			"properties": mi{
				"integer": mi{"type": "integer"},
				"long":    mi{"type": "long"},
				"keyword": mi{"type": "keyword"},
				"date":    mi{"type": "date", "format": "yyyy-MM-dd HH:mm:ss"},
				"text": mi{
					"type":            "text",
					"analyzer":        "ik_max_word",
					"search_analyzer": "ik_max_word",
				},
				"goodsArea": mi{"type": "integer"},
				"goodsAreas": mi{
					"properties": mi{
						"area": mi{
							"type": "integer",
						},
					},
				},
				"userArea": mi{"type": "integer"},
				"userAreas": mi{
					"properties": mi{
						"area": mi{
							"type": "integer",
						},
					},
				},
				"nested": mi{
					"type": "nested",
					"properties": mi{
						"integer": mi{"type": "integer"},
						"long":    mi{"type": "long"},
						"keyword": mi{"type": "keyword"},
						"date":    mi{"type": "date", "format": "yyyy-MM-dd HH:mm:ss"},
						"text": mi{
							"type":            "text",
							"analyzer":        "ik_max_word",
							"search_analyzer": "ik_max_word",
						},
						"goodsArea": mi{"type": "integer"},
						"goodsAreas": mi{
							"properties": mi{
								"area": mi{
									"type": "integer",
								},
							},
						},
						"userArea": mi{"type": "integer"},
						"userAreas": mi{
							"properties": mi{
								"area": mi{
									"type": "integer",
								},
							},
						},
					},
				},
				"obj": mi{
					"properties": mi{
						"integer": mi{"type": "integer"},
						"long":    mi{"type": "long"},
						"keyword": mi{"type": "keyword"},
						"date":    mi{"type": "date", "format": "yyyy-MM-dd HH:mm:ss"},
						"text": mi{
							"type":            "text",
							"analyzer":        "ik_max_word",
							"search_analyzer": "ik_max_word",
						},
						"goodsArea": mi{"type": "integer"},
						"goodsAreas": mi{
							"properties": mi{
								"area": mi{
									"type": "integer",
								},
							},
						},
						"userArea": mi{"type": "integer"},
						"userAreas": mi{
							"properties": mi{
								"area": mi{
									"type": "integer",
								},
							},
						},
					},
				},
			},
		},
	},
}
