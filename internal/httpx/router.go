package httpx

import (
	"context"
	"net/http"
	"strings"
)

// Router 极简路由器：支持 /a/b/{id} 形式的单段路径参数。
// go.mod 语言版本为 go 1.21，不能使用 http.ServeMux 的增强模式，故自行实现。
type Router struct {
	routes []route
}

type route struct {
	method   string
	segments []string
	handler  http.HandlerFunc
}

// NewRouter 创建路由器。
func NewRouter() *Router { return &Router{} }

// Handle 注册路由，pattern 形如 "GET /api/v1/artifacts/{id}"。
func (rt *Router) Handle(pattern string, h http.HandlerFunc) {
	parts := strings.SplitN(strings.TrimSpace(pattern), " ", 2)
	if len(parts) != 2 {
		panic("非法路由模式: " + pattern)
	}
	segs := splitPath(parts[1])
	rt.routes = append(rt.routes, route{method: parts[0], segments: segs, handler: h})
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// PathValue 从请求上下文取路径参数。
func PathValue(r *http.Request, name string) string {
	if v, ok := r.Context().Value(pathKey(name)).(string); ok {
		return v
	}
	return ""
}

type pathKey string

// ServeHTTP 实现 http.Handler。
func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	segs := splitPath(r.URL.Path)
	for _, rt0 := range rt.routes {
		if rt0.method != r.Method || len(rt0.segments) != len(segs) {
			continue
		}
		vars := map[string]string{}
		match := true
		for i, seg := range rt0.segments {
			if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
				vars[seg[1:len(seg)-1]] = segs[i]
			} else if seg != segs[i] {
				match = false
				break
			}
		}
		if match {
			ctx := r.Context()
			for k, v := range vars {
				ctx = context.WithValue(ctx, pathKey(k), v)
			}
			rt0.handler(w, r.WithContext(ctx))
			return
		}
	}
	WriteJSON(w, http.StatusNotFound, map[string]any{
		"error": map[string]string{"code": "not_found", "message": "路由不存在: " + r.Method + " " + r.URL.Path},
	})
}
