package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEngineRoutesWithParamsAndMiddleware(t *testing.T) {
	engine := New()
	engine.Use(func(next HandlerFunc) HandlerFunc {
		return func(c *Context) {
			c.Writer.Header().Set("X-Test", "yes")
			next(c)
		}
	})
	engine.GET("/users/:id", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
	})

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	rr := httptest.NewRecorder()
	engine.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("X-Test") != "yes" {
		t.Fatalf("X-Test header = %q, want yes", rr.Header().Get("X-Test"))
	}
}

func TestCORSHandlesOptions(t *testing.T) {
	engine := New()
	engine.Use(CORS(CORSConfig{AllowOrigins: []string{"https://example.com"}}))
	engine.GET("/ping", func(c *Context) {
		c.String(http.StatusOK, "pong")
	})

	req := httptest.NewRequest(http.MethodOptions, "/ping", nil)
	req.Header.Set("Origin", "https://example.com")
	rr := httptest.NewRecorder()
	engine.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Fatalf("allow origin = %q", rr.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestBearerAuth(t *testing.T) {
	engine := New()
	engine.Use(BearerAuth(AuthConfig{Token: "secret"}))
	engine.GET("/secure", func(c *Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	rr := httptest.NewRecorder()
	engine.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want 401", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rr = httptest.NewRecorder()
	engine.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("authorized status = %d, want 200", rr.Code)
	}
}
