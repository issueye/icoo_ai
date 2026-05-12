package httpx

import (
	"net/http"
	"strings"
)

type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           string
}

func CORS(config CORSConfig) Middleware {
	allowMethods := joinHeader(config.AllowMethods, "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	allowHeaders := joinHeader(config.AllowHeaders, "Authorization, Content-Type")
	allowOrigins := append([]string(nil), config.AllowOrigins...)
	if len(allowOrigins) == 0 {
		allowOrigins = []string{"*"}
	}
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) {
			origin := c.Request.Header.Get("Origin")
			if allowedOrigin := resolveOrigin(origin, allowOrigins); allowedOrigin != "" {
				c.Writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			}
			c.Writer.Header().Set("Access-Control-Allow-Methods", allowMethods)
			c.Writer.Header().Set("Access-Control-Allow-Headers", allowHeaders)
			if len(config.ExposeHeaders) > 0 {
				c.Writer.Header().Set("Access-Control-Expose-Headers", strings.Join(config.ExposeHeaders, ", "))
			}
			if config.AllowCredentials {
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if config.MaxAge != "" {
				c.Writer.Header().Set("Access-Control-Max-Age", config.MaxAge)
			}
			if c.Request.Method == http.MethodOptions {
				c.Status(http.StatusNoContent)
				return
			}
			next(c)
		}
	}
}

type AuthConfig struct {
	Token     string
	Validator func(token string, r *http.Request) bool
	Skipper   func(r *http.Request) bool
}

func BearerAuth(config AuthConfig) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) {
			if config.Skipper != nil && config.Skipper(c.Request) {
				next(c)
				return
			}
			token := bearerToken(c.Request.Header.Get("Authorization"))
			valid := false
			if config.Validator != nil {
				valid = config.Validator(token, c.Request)
			} else {
				valid = config.Token != "" && token == config.Token
			}
			if !valid {
				c.Writer.Header().Set("WWW-Authenticate", "Bearer")
				c.Abort(http.StatusUnauthorized, map[string]string{"code": "unauthorized", "message": "missing or invalid bearer token"})
				return
			}
			next(c)
		}
	}
}

func Recovery() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) {
			defer func() {
				if recover() != nil {
					c.JSON(http.StatusInternalServerError, map[string]string{"code": "internal_error", "message": "internal server error"})
				}
			}()
			next(c)
		}
	}
}

func joinHeader(values []string, fallback string) string {
	if len(values) == 0 {
		return fallback
	}
	return strings.Join(values, ", ")
}

func resolveOrigin(origin string, allowed []string) string {
	for _, item := range allowed {
		item = strings.TrimSpace(item)
		if item == "*" {
			return "*"
		}
		if origin != "" && item == origin {
			return origin
		}
	}
	return ""
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}
