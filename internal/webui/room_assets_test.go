package webui

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestPhotographicRoomAssetDelivery(t *testing.T) {
	root := t.TempDir()
	h, err := Handler(root, root, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"cozy-beige-room-photo-1920.webp", "cozy-beige-room-photo-3840.webp"} {
		t.Run(name, func(t *testing.T) {
			get := httptest.NewRecorder()
			h.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/"+name, nil))
			if get.Code != http.StatusOK {
				t.Fatalf("GET: %d", get.Code)
			}
			body := get.Body.Bytes()
			if len(body) < 12 || string(body[:4]) != "RIFF" || string(body[8:12]) != "WEBP" {
				t.Fatal("response is not WebP")
			}
			hash := fmt.Sprintf("%x", sha256.Sum256(body))
			etag := "\"" + hash + "\""
			if get.Header().Get("Content-Type") != "image/webp" || get.Header().Get("ETag") != etag || get.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("image MIME/integrity headers missing")
			}
			if get.Header().Get("Content-Length") != strconv.Itoa(len(body)) {
				t.Fatal("incorrect image length")
			}
			if strings.Contains(get.Header().Get("Cache-Control"), "immutable") {
				t.Fatal("unversioned image must revalidate")
			}
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				r := httptest.NewRecorder()
				h.ServeHTTP(r, httptest.NewRequest(method, "/"+name+"?v="+hash[:8], nil))
				if r.Code != http.StatusOK || !strings.Contains(r.Header().Get("Cache-Control"), "immutable") {
					t.Fatal("versioned image is not cacheable")
				}
				if method == http.MethodHead && r.Body.Len() != 0 {
					t.Fatal("HEAD returned image bytes")
				}
			}
			req := httptest.NewRequest(http.MethodGet, "/"+name, nil)
			req.Header.Set("If-None-Match", etag)
			cached := httptest.NewRecorder()
			h.ServeHTTP(cached, req)
			if cached.Code != http.StatusNotModified || cached.Body.Len() != 0 {
				t.Fatal("conditional image GET did not revalidate")
			}
			wrong := httptest.NewRecorder()
			h.ServeHTTP(wrong, httptest.NewRequest(http.MethodPost, "/"+name, nil))
			if wrong.Code != http.StatusMethodNotAllowed {
				t.Fatal("image route accepted mutation")
			}
		})
	}
}
