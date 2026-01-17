package middleware

import "net/http"

func SecureHeaders(isProd bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			headers := w.Header()
			headers.Set("X-Content-Type-Options", "nosniff")
			headers.Set("X-Frame-Options", "DENY")
			headers.Set("Referrer-Policy", "no-referrer")
			headers.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=()")
			headers.Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; object-src 'none'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'")
			headers.Set("Cross-Origin-Opener-Policy", "same-origin")
			headers.Set("Cross-Origin-Resource-Policy", "same-origin")
			if isProd {
				headers.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
			}
			next.ServeHTTP(w, r)
		})
	}
}
