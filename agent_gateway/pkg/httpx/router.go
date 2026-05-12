package httpx

import (
	"net/http"
	"strings"
)

type Middleware func(HandlerFunc) HandlerFunc

type Engine struct {
	routes     []route
	middleware []Middleware
	NotFound   HandlerFunc
}

type Group struct {
	engine     *Engine
	prefix     string
	middleware []Middleware
}

type route struct {
	method      string
	pattern     string
	segments    []string
	handler     HandlerFunc
	middleware  []Middleware
	catchPrefix bool
}

func New() *Engine {
	return &Engine{
		NotFound: func(c *Context) {
			c.JSON(http.StatusNotFound, map[string]string{"code": "not_found", "message": "route not found"})
		},
	}
}

func (e *Engine) Use(middleware ...Middleware) {
	e.middleware = append(e.middleware, middleware...)
}

func (e *Engine) Group(prefix string, middleware ...Middleware) *Group {
	return &Group{
		engine:     e,
		prefix:     cleanPath(prefix),
		middleware: append([]Middleware(nil), middleware...),
	}
}

func (e *Engine) Handle(method, path string, handler HandlerFunc, middleware ...Middleware) {
	e.add(method, path, false, handler, middleware)
}

func (e *Engine) HandlePrefix(method, path string, handler HandlerFunc, middleware ...Middleware) {
	e.add(method, path, true, handler, middleware)
}

func (e *Engine) GET(path string, handler HandlerFunc, middleware ...Middleware) {
	e.Handle(http.MethodGet, path, handler, middleware...)
}

func (e *Engine) POST(path string, handler HandlerFunc, middleware ...Middleware) {
	e.Handle(http.MethodPost, path, handler, middleware...)
}

func (e *Engine) PUT(path string, handler HandlerFunc, middleware ...Middleware) {
	e.Handle(http.MethodPut, path, handler, middleware...)
}

func (e *Engine) DELETE(path string, handler HandlerFunc, middleware ...Middleware) {
	e.Handle(http.MethodDelete, path, handler, middleware...)
}

func (e *Engine) PATCH(path string, handler HandlerFunc, middleware ...Middleware) {
	e.Handle(http.MethodPatch, path, handler, middleware...)
}

func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	optionsPathMatched := false
	for _, route := range e.routes {
		params, ok := match(route, r.URL.Path)
		if !ok {
			continue
		}
		if route.method != r.Method {
			if r.Method == http.MethodOptions {
				optionsPathMatched = true
			}
			continue
		}
		chain := buildChain(route.handler, append(e.middleware, route.middleware...))
		ctx := &Context{Writer: w, Request: r, params: params, index: -1, chain: []HandlerFunc{chain}}
		ctx.Next()
		return
	}
	if optionsPathMatched {
		chain := buildChain(func(c *Context) {}, e.middleware)
		ctx := &Context{Writer: w, Request: r, index: -1, chain: []HandlerFunc{chain}}
		ctx.Next()
		return
	}
	ctx := &Context{Writer: w, Request: r, index: -1, chain: []HandlerFunc{e.NotFound}}
	ctx.Next()
}

func (g *Group) Use(middleware ...Middleware) {
	g.middleware = append(g.middleware, middleware...)
}

func (g *Group) Group(prefix string, middleware ...Middleware) *Group {
	merged := append([]Middleware(nil), g.middleware...)
	merged = append(merged, middleware...)
	return &Group{
		engine:     g.engine,
		prefix:     joinPath(g.prefix, prefix),
		middleware: merged,
	}
}

func (g *Group) Handle(method, path string, handler HandlerFunc, middleware ...Middleware) {
	g.engine.add(method, joinPath(g.prefix, path), false, handler, append(g.middleware, middleware...))
}

func (g *Group) HandlePrefix(method, path string, handler HandlerFunc, middleware ...Middleware) {
	g.engine.add(method, joinPath(g.prefix, path), true, handler, append(g.middleware, middleware...))
}

func (g *Group) GET(path string, handler HandlerFunc, middleware ...Middleware) {
	g.Handle(http.MethodGet, path, handler, middleware...)
}

func (g *Group) POST(path string, handler HandlerFunc, middleware ...Middleware) {
	g.Handle(http.MethodPost, path, handler, middleware...)
}

func (g *Group) PUT(path string, handler HandlerFunc, middleware ...Middleware) {
	g.Handle(http.MethodPut, path, handler, middleware...)
}

func (g *Group) DELETE(path string, handler HandlerFunc, middleware ...Middleware) {
	g.Handle(http.MethodDelete, path, handler, middleware...)
}

func (g *Group) PATCH(path string, handler HandlerFunc, middleware ...Middleware) {
	g.Handle(http.MethodPatch, path, handler, middleware...)
}

func (e *Engine) add(method, path string, catchPrefix bool, handler HandlerFunc, middleware []Middleware) {
	pattern := cleanPath(path)
	e.routes = append(e.routes, route{
		method:      method,
		pattern:     pattern,
		segments:    splitSegments(pattern),
		handler:     handler,
		middleware:  append([]Middleware(nil), middleware...),
		catchPrefix: catchPrefix,
	})
}

func buildChain(final HandlerFunc, middleware []Middleware) HandlerFunc {
	handler := final
	for i := len(middleware) - 1; i >= 0; i-- {
		handler = middleware[i](handler)
	}
	return handler
}

func match(route route, path string) (map[string]string, bool) {
	target := splitSegments(cleanPath(path))
	if !route.catchPrefix && len(target) != len(route.segments) {
		return nil, false
	}
	if route.catchPrefix && len(target) < len(route.segments) {
		return nil, false
	}
	params := map[string]string{}
	for i, segment := range route.segments {
		if i >= len(target) {
			return nil, false
		}
		if strings.HasPrefix(segment, ":") {
			params[strings.TrimPrefix(segment, ":")] = target[i]
			continue
		}
		if segment != target[i] {
			return nil, false
		}
	}
	return params, true
}

func cleanPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	path = strings.TrimRight(path, "/")
	if path == "" {
		return "/"
	}
	return path
}

func joinPath(prefix, path string) string {
	if cleanPath(prefix) == "/" {
		return cleanPath(path)
	}
	if cleanPath(path) == "/" {
		return cleanPath(prefix)
	}
	return cleanPath(prefix) + cleanPath(path)
}

func splitSegments(path string) []string {
	path = cleanPath(path)
	if path == "/" {
		return nil
	}
	return strings.Split(strings.Trim(path, "/"), "/")
}
