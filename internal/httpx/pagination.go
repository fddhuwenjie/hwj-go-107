package httpx

import (
	"net/http"
	"strconv"
	"strings"

	"gowork/internal/domain"
)

// ParsePage 解析键集分页参数 cursor/limit。
func ParsePage(r *http.Request) (domain.Page, error) {
	q := r.URL.Query()
	var p domain.Page
	if c := q.Get("cursor"); c != "" {
		v, err := strconv.ParseInt(c, 10, 64)
		if err != nil || v < 0 {
			return p, domain.Invalidf("非法分页游标 %q", c)
		}
		p.Cursor = v
	}
	if l := q.Get("limit"); l != "" {
		v, err := strconv.Atoi(l)
		if err != nil || v <= 0 || v > 200 {
			return p, domain.Invalidf("非法分页大小 %q（1..200）", l)
		}
		p.Limit = v
	}
	return p.Normalize(), nil
}

// ParseID 解析路径中的数字 id。
func ParseID(s string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || id <= 0 {
		return 0, domain.Invalidf("非法 id %q", s)
	}
	return id, nil
}

// QueryString 取查询参数，空则返回 nil。
func QueryString(r *http.Request, key string) *string {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	return &v
}

// QueryInt64 取整数查询参数。
func QueryInt64(r *http.Request, key string) (*int64, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return nil, domain.Invalidf("非法整数参数 %s=%q", key, v)
	}
	return &n, nil
}
