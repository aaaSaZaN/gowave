package subscription

import (
	"crypto/sha1"
	"encoding/base64"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"gowave/internal/subtpl/ojson"
)

// The public subscription routes answer exactly like upstream's Express 5
// stack: res.send adds a charset to text types, a weak ETag and conditional
// GET handling.

// expressETag ports the etag package for a string body.
func expressETag(body []byte) string {
	if len(body) == 0 {
		return `W/"0-2jmj7l5rSw0yVb/vlWAYkK/YBwk"`
	}
	sum := sha1.Sum(body)
	return `W/"` + strconv.FormatInt(int64(len(body)), 16) + "-" + base64.StdEncoding.EncodeToString(sum[:])[:27] + `"`
}

var noCachePattern = regexp.MustCompile(`(?:^|,)\s*?no-cache\s*?(?:,|$)`)

// expressFresh ports the fresh package (ETag validators only; Express sets
// no Last-Modified here).
func expressFresh(r *http.Request, etag, lastModified string) bool {
	modifiedSince := r.Header.Get("If-Modified-Since")
	noneMatch := r.Header.Get("If-None-Match")
	if modifiedSince == "" && noneMatch == "" {
		return false
	}
	if cacheControl := r.Header.Get("Cache-Control"); cacheControl != "" && noCachePattern.MatchString(cacheControl) {
		return false
	}
	if noneMatch != "" && noneMatch != "*" {
		if etag == "" {
			return false
		}
		stale := true
		for _, match := range strings.FieldsFunc(noneMatch, func(r rune) bool { return r == ' ' || r == ',' }) {
			if match == etag || match == "W/"+etag || "W/"+match == etag {
				stale = false
				break
			}
		}
		if stale {
			return false
		}
	}
	if modifiedSince != "" {
		last, err1 := http.ParseTime(lastModified)
		since, err2 := http.ParseTime(modifiedSince)
		if lastModified == "" || err1 != nil || err2 != nil || last.After(since) {
			return false
		}
	}
	return true
}

// withCharset ports Express' setCharset(type, 'utf-8').
func withCharset(contentType string) string {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return contentType
	}
	params["charset"] = "utf-8"
	return mime.FormatMediaType(mediaType, params)
}

// expressType ports res.type: bare extensions go through the mime table.
func expressType(contentType string) string {
	if strings.Contains(contentType, "/") {
		return contentType
	}
	if found := mime.TypeByExtension("." + strings.TrimPrefix(contentType, ".")); found != "" {
		return found
	}
	return "application/octet-stream"
}

// expressSend ports res.status(status).type(contentType).send(string).
func expressSend(w http.ResponseWriter, r *http.Request, status int, contentType string, body string) {
	header := w.Header()
	if contentType != "" {
		header.Set("Content-Type", withCharset(expressType(contentType)))
	} else if existing := header.Get("Content-Type"); existing != "" {
		header.Set("Content-Type", withCharset(existing))
	} else {
		header.Set("Content-Type", "text/html; charset=utf-8")
	}
	payload := []byte(body)
	if header.Get("ETag") == "" {
		header.Set("ETag", expressETag(payload))
	}
	if (r.Method == http.MethodGet || r.Method == http.MethodHead) && (status >= 200 && status < 300 || status == http.StatusNotModified) &&
		expressFresh(r, header.Get("ETag"), header.Get("Last-Modified")) {
		status = http.StatusNotModified
	}
	if status == http.StatusNoContent || status == http.StatusNotModified {
		header.Del("Content-Type")
		header.Del("Content-Length")
		header.Del("Transfer-Encoding")
		payload = nil
	} else {
		header.Set("Content-Length", strconv.Itoa(len(payload)))
	}
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(payload)
	}
}

// expressJSON ports res.status(status).json(value) for ojson values.
func expressJSON(w http.ResponseWriter, r *http.Request, status int, value any) {
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	expressSend(w, r, status, "", ojson.Stringify(value))
}

// nestError ports Nest's BaseExceptionFilter reply for an HttpException
// with a string message.
func nestError(w http.ResponseWriter, r *http.Request, status int, message string) {
	expressJSON(w, r, status, ojson.New("statusCode", float64(status), "message", message))
}
