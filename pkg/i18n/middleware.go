package i18n

import (
	"net/http"
)

// Middleware extracts locale from Accept-Language header and adds it to context
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse Accept-Language header
		acceptLang := r.Header.Get("Accept-Language")
		locale := ParseAcceptLanguage(acceptLang)

		// Add locale to context
		ctx := WithLocale(r.Context(), locale)

		// Call next handler with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// MiddlewareFunc is the same as Middleware but returns a HandlerFunc
func MiddlewareFunc(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acceptLang := r.Header.Get("Accept-Language")
		locale := ParseAcceptLanguage(acceptLang)
		ctx := WithLocale(r.Context(), locale)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
