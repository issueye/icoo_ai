package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

type HandlerFunc func(*Context)

type Context struct {
	Writer  http.ResponseWriter
	Request *http.Request

	params map[string]string
	index  int
	chain  []HandlerFunc
}

func (c *Context) Next() {
	c.index++
	for c.index < len(c.chain) {
		handler := c.chain[c.index]
		handler(c)
		c.index++
	}
}

func (c *Context) Param(name string) string {
	if c.params == nil {
		return ""
	}
	return c.params[name]
}

func (c *Context) Query(name string) string {
	return strings.TrimSpace(c.Request.URL.Query().Get(name))
}

func (c *Context) Header(name string) string {
	return c.Request.Header.Get(name)
}

func (c *Context) BindJSON(dst any) error {
	defer c.Request.Body.Close()
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}

func (c *Context) JSON(status int, value any) {
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(status)
	_ = json.NewEncoder(c.Writer).Encode(value)
}

func (c *Context) String(status int, text string) {
	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Writer.WriteHeader(status)
	_, _ = c.Writer.Write([]byte(text))
}

func (c *Context) Status(status int) {
	c.Writer.WriteHeader(status)
}

func (c *Context) Abort(status int, value any) {
	c.JSON(status, value)
	c.index = len(c.chain)
}
