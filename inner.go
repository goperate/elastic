package es

import "github.com/olivere/elastic"

type InnerHits interface {
	InitInnerHits() *elastic.InnerHit
}

type Select struct {
	Page    int          `json:"page"`
	Size    int          `json:"size"`
	Include ArrayKeyword `json:"include"` //返回的字段
	Exclude ArrayKeyword `json:"exclude"` //忽略的字段
}

func (t *Select) InitInnerHits() (res *elastic.InnerHit) {
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
