package middleware

import "net/http"

// SecurityHeaders adds standard security headers to all responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' https://accounts.google.com; "+
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://accounts.google.com; "+
				"font-src 'self' https://fonts.gstatic.com; "+
				// image.tmdb.org, s4.anilist.co, artworks.thetvdb.com : jaquettes distantes
				// (recherche TMDB, relations AniList) affichées directement depuis le CDN.
				"img-src 'self' https://lh3.googleusercontent.com https://image.tmdb.org https://s4.anilist.co https://*.anilist.co https://artworks.thetvdb.com data:; "+
				"connect-src 'self' https://accounts.google.com; "+
				"frame-src https://accounts.google.com; "+
				"worker-src 'self'; "+
				"manifest-src 'self'")
		next.ServeHTTP(w, r)
	})
}
