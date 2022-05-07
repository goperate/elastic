package basics

import (
	"github.com/olivere/elastic"
)

type EsInnerHits interface {
	GetPage() int
	GetSize() int
	GetInclude() []string
	GetExclude() []string

	SetSource(req *elastic.SearchService)
	InitInnerHits() *elastic.InnerHit
}

type EsSelect struct {
	Page    int          `json:"page"`
	Size    int          `json:"size"`
	Include ArrayKeyword `json:"include"` //返回的字段
	Exclude ArrayKeyword `json:"exclude"` //忽略的字段
}

func (t *EsSelect) GetPage() int {
	return t.Page
}

func (t *EsSelect) GetSize() int {
	return t.Size
}

func (t *EsSelect) GetInclude() []string {
	return t.Include
}

func (t *EsSelect) GetExclude() []string {
	return t.Exclude
}

func (t *EsSelect) SetSource(req *elastic.SearchService) {
	if t.Page == 0 {
		t.Page = 1
	}
	if t.Size == 0 {
		t.Size = 10
	}
	req.From(t.Page*t.Size - t.Size).Size(t.Size)
	if len(t.Include)+len(t.Exclude) > 0 {
		req.FetchSourceContext(
			elastic.NewFetchSourceContext(true).Include(t.Include...).Exclude(t.Exclude...),
		)
	}
}

func (t *EsSelect) InitInnerHits() (res *elastic.InnerHit) {
	res = elastic.NewInnerHit()
	if t.Page == 0 {
		t.Page = 1
	}
	if t.Size == 0 {
		t.Size = 100
	}
	res = res.From(t.Page*t.Size - t.Size)
	res = res.Size(t.Size)
	if len(t.Include)+len(t.Exclude) > 0 {
		res = res.FetchSourceContext(
			elastic.NewFetchSourceContext(true).Include(t.Include...).Exclude(t.Exclude...),
		)
	}
	return
}
