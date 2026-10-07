package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"echoes.com/m/internal/s3fake"
	"github.com/gofiber/fiber/v3"
)

var (
	jpegBytes = append([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0x10, 'J', 'F', 'I', 'F', 0}, make([]byte, 600)...)
	pngBytes  = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 100)...)
	avifBytes = append([]byte{0, 0, 0, 0x1c, 'f', 't', 'y', 'p', 'a', 'v', 'i', 'f', 0, 0, 0, 0, 'a', 'v', 'i', 'f', 'm', 'i', 'f', '1'}, make([]byte, 50)...)
	heicBytes = append([]byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'h', 'e', 'i', 'c', 0, 0, 0, 0, 'm', 'i', 'f', '1', 'h', 'e', 'i', 'c'}, make([]byte, 50)...)
	svgBytes  = []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
)

// newTestApp mounts the photo routes without FirebaseAuth/RequireAdmin
// (covered in the middlewares package), with a fixed signed-in user.
func newTestApp(t *testing.T) *fiber.App {
	t.Helper()
	s3fake.Start(t)

	app := fiber.New(fiber.Config{BodyLimit: 25 << 20})
	app.Use(func(c fiber.Ctx) error {
		c.Locals("user_email", "mamie@example.com")
		return c.Next()
	})
	app.Get("/api/albums", HandlerGetAlbums)
	app.Patch("/api/albums/:id", HandlerRenameAlbum)
	app.Get("/api/albums/:id/photos", HandlerGetPhotos)
	app.Post("/api/albums/:id/photos", HandlerUploadPhoto)
	app.Delete("/api/albums/:id/photos/:photoId", HandlerDeletePhoto)
	return app
}

type formFile struct {
	field, name, contentType string
	data                     []byte
}

func send(t *testing.T, app *fiber.App, req *http.Request) (int, map[string]any) {
	t.Helper()
	res, err := app.Test(req, fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	var decoded map[string]any
	_ = json.Unmarshal(body, &decoded)
	return res.StatusCode, decoded
}

func upload(t *testing.T, app *fiber.App, album string, files ...formFile) (int, map[string]any) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, f := range files {
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition", `form-data; name="`+f.field+`"; filename="`+f.name+`"`)
		header.Set("Content-Type", f.contentType)
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		part.Write(f.data)
	}
	writer.Close()

	req := httptest.NewRequest("POST", "/api/albums/"+album+"/photos", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return send(t, app, req)
}

func photo(name, contentType string, data []byte) formFile {
	return formFile{field: "photo", name: name, contentType: contentType, data: data}
}

func TestUploadChecksActualFileType(t *testing.T) {
	app := newTestApp(t)

	cases := []struct {
		name    string
		file    formFile
		want    int
		wantExt string
	}{
		{"jpeg with a generic content type", photo("été.jpg", "application/octet-stream", jpegBytes), 201, ".jpg"},
		{"png with a misleading filename", photo("photo.jpg", "image/jpeg", pngBytes), 201, ".png"},
		{"avif", photo("a.avif", "image/avif", avifBytes), 201, ".avif"},
		{"svg claiming to be jpeg", photo("x.jpg", "image/jpeg", svgBytes), 415, ""},
		{"heic", photo("x.heic", "image/heic", heicBytes), 415, ""},
		{"empty file", photo("x.jpg", "image/jpeg", nil), 415, ""},
		{"over 20MB", photo("big.jpg", "image/jpeg", append(jpegBytes, make([]byte, 20<<20)...)), 413, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, body := upload(t, app, "Vacances", tc.file)
			if code != tc.want {
				t.Fatalf("status = %d, want %d (%v)", code, tc.want, body)
			}
			if tc.wantExt != "" {
				url, _ := body["url"].(string)
				path, _, _ := strings.Cut(url, "?")
				if !strings.HasSuffix(path, tc.wantExt) {
					t.Errorf("stored as %q, want extension %s", path, tc.wantExt)
				}
			}
		})
	}
}

func TestUploadDecodesAndValidatesAlbumName(t *testing.T) {
	app := newTestApp(t)

	code, body := upload(t, app, "%C3%89t%C3%A9%202024", photo("a.jpg", "image/jpeg", jpegBytes))
	if code != 201 || body["album_name"] != "Été 2024" {
		t.Errorf("encoded album name: status %d, album_name %v", code, body["album_name"])
	}

	for _, album := range []string{"a%2Fb", "%2E%2E", "%20"} {
		if code, _ := upload(t, app, album, photo("a.jpg", "image/jpeg", jpegBytes)); code != 400 {
			t.Errorf("album %q: status %d, want 400", album, code)
		}
	}
}

func TestUploadThumbnailIsOptionalAndBestEffort(t *testing.T) {
	app := newTestApp(t)

	cases := []struct {
		name      string
		thumb     *formFile
		wantThumb bool
	}{
		{"valid thumbnail", &formFile{"thumbnail", "t.jpg", "image/jpeg", jpegBytes}, true},
		{"no thumbnail", nil, false},
		{"invalid thumbnail", &formFile{"thumbnail", "t.jpg", "image/jpeg", svgBytes}, false},
		// PNG is a valid photo type but not one the client ever produces for
		// thumbnails.
		{"png thumbnail", &formFile{"thumbnail", "t.png", "image/png", pngBytes}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := []formFile{photo("a.jpg", "image/jpeg", jpegBytes)}
			if tc.thumb != nil {
				files = append(files, *tc.thumb)
			}
			code, body := upload(t, app, "Vacances", files...)
			if code != 201 {
				t.Fatalf("status = %d, want 201 (%v)", code, body)
			}
			if _, hasThumb := body["thumb_url"]; hasThumb != tc.wantThumb {
				t.Errorf("thumb_url present = %v, want %v", hasThumb, tc.wantThumb)
			}
		})
	}
}

func TestGetAlbumsUsesThumbnailAsCover(t *testing.T) {
	app := newTestApp(t)

	upload(t, app, "Vacances", photo("a.jpg", "image/jpeg", jpegBytes),
		formFile{"thumbnail", "t.jpg", "image/jpeg", jpegBytes})
	upload(t, app, "Vacances", photo("b.jpg", "image/jpeg", jpegBytes))
	upload(t, app, "Noël", photo("c.jpg", "image/jpeg", jpegBytes))

	code, body := send(t, app, httptest.NewRequest("GET", "/api/albums", nil))
	if code != 200 {
		t.Fatalf("status = %d", code)
	}

	albums, _ := body["albums"].([]any)
	if len(albums) != 2 {
		t.Fatalf("got %d albums, want 2: %v", len(albums), body)
	}
	vacances := albums[0].(map[string]any)
	if vacances["name"] != "Vacances" || vacances["count"] != float64(2) {
		t.Errorf("first album = %v", vacances)
	}
	if cover, _ := vacances["cover_url"].(string); !strings.Contains(cover, "/thumbs/") {
		t.Errorf("cover_url = %q, want the thumbnail", cover)
	}
}

func TestDeletePhotoStatuses(t *testing.T) {
	app := newTestApp(t)

	_, created := upload(t, app, "Vacances", photo("a.jpg", "image/jpeg", jpegBytes))
	id, _ := created["id"].(string)

	cases := []struct {
		name, path string
		want       int
	}{
		{"delete", "/api/albums/Vacances/photos/" + id, 204},
		{"already deleted", "/api/albums/Vacances/photos/" + id, 404},
		{"not a uuid", "/api/albums/Vacances/photos/not-a-uuid", 400},
		{"invalid album", "/api/albums/a%2Fb/photos/" + id, 400},
	}
	for _, tc := range cases {
		if code, _ := send(t, app, httptest.NewRequest("DELETE", tc.path, nil)); code != tc.want {
			t.Errorf("%s: status %d, want %d", tc.name, code, tc.want)
		}
	}
}

func TestRenameAlbumStatuses(t *testing.T) {
	app := newTestApp(t)

	upload(t, app, "Vacances", photo("a.jpg", "image/jpeg", jpegBytes))
	upload(t, app, "Noël", photo("b.jpg", "image/jpeg", jpegBytes))

	rename := func(from, body string) int {
		req := httptest.NewRequest("PATCH", "/api/albums/"+from, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		code, _ := send(t, app, req)
		return code
	}

	cases := []struct {
		name, from, body string
		want             int
	}{
		{"onto existing album", "Vacances", `{"name":"Noël"}`, 409},
		{"invalid new name", "Vacances", `{"name":"a/b"}`, 400},
		{"empty new name", "Vacances", `{"name":"  "}`, 400},
		{"missing album", "Absent", `{"name":"Autre"}`, 404},
		{"malformed body", "Vacances", `{`, 400},
		{"rename", "Vacances", `{"name":"Été 2024"}`, 200},
	}
	for _, tc := range cases {
		if code := rename(tc.from, tc.body); code != tc.want {
			t.Errorf("%s: status %d, want %d", tc.name, code, tc.want)
		}
	}

	code, body := send(t, app, httptest.NewRequest("GET", "/api/albums/%C3%89t%C3%A9%202024/photos", nil))
	if code != 200 || body["total"] != float64(1) {
		t.Errorf("renamed album: status %d, body %v", code, body)
	}
}
