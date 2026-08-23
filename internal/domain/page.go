package domain

// Page 键集分页请求。
type Page struct {
	// Cursor 上一页最后一条记录的主键，0 表示第一页。
	Cursor int64
	// Limit 每页条数，取值 1..200。
	Limit int
}

// Normalize 规范化分页参数。
func (p Page) Normalize() Page {
	if p.Limit <= 0 || p.Limit > 200 {
		p.Limit = 50
	}
	if p.Cursor < 0 {
		p.Cursor = 0
	}
	return p
}

// Paged 键集分页结果。
type Paged[T any] struct {
	// Items 当前页数据。
	Items []T `json:"items"`
	// NextCursor 下一页游标，0 表示没有更多。
	NextCursor int64 `json:"next_cursor"`
}

// BuildPaged 依据查询结果构造分页结果，fetch 时须多取一条判断是否还有下一页。
func BuildPaged[T any](items []T, limit int, idOf func(T) int64) Paged[T] {
	out := Paged[T]{Items: items}
	if len(items) > limit {
		out.Items = items[:limit]
		out.NextCursor = idOf(items[limit-1])
	}
	if out.Items == nil {
		out.Items = []T{}
	}
	return out
}
