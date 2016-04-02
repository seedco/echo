package middleware

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"testing"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/engine/standard"
	"github.com/labstack/echo/test"
	"github.com/stretchr/testify/assert"
)

func TestGzip(t *testing.T) {
	e := echo.New()
	rq := test.NewRequest(echo.GET, "/", nil)
	rec := test.NewResponseRecorder()
	c := echo.NewContext(rq, rec, e)

	// Skip if no Accept-Encoding header
	h := Gzip()(echo.HandlerFunc(func(c echo.Context) error {
		c.Response().Write([]byte("test")) // For Content-Type sniffing
		return nil
	}))
	h.Handle(c)
	assert.Equal(t, "test", rec.Body.String())

	rq = test.NewRequest(echo.GET, "/", nil)
	rq.Header().Set(echo.AcceptEncoding, "gzip")
	rec = test.NewResponseRecorder()
	c = echo.NewContext(rq, rec, e)

	// Gzip
	h.Handle(c)
	assert.Equal(t, "gzip", rec.Header().Get(echo.ContentEncoding))
	assert.Contains(t, rec.Header().Get(echo.ContentType), echo.TextPlain)
	r, err := gzip.NewReader(rec.Body)
	defer r.Close()
	if assert.NoError(t, err) {
		buf := new(bytes.Buffer)
		buf.ReadFrom(r)
		assert.Equal(t, "test", buf.String())
	}
}

func TestGzipNoContent(t *testing.T) {
	e := echo.New()
	rq := test.NewRequest(echo.GET, "/", nil)
	rec := test.NewResponseRecorder()
	c := echo.NewContext(rq, rec, e)
	h := Gzip()(echo.HandlerFunc(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	}))
	h.Handle(c)

	assert.Empty(t, rec.Header().Get(echo.ContentEncoding))
	assert.Empty(t, rec.Header().Get(echo.ContentType))
	b, err := ioutil.ReadAll(rec.Body)
	if assert.NoError(t, err) {
		assert.Equal(t, 0, len(b))
	}
}

func TestGzipErrorReturned(t *testing.T) {
	e := echo.New()
	e.Use(Gzip())
	e.Get("/", echo.HandlerFunc(func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusInternalServerError, "error")
	}))
	rq := test.NewRequest(echo.GET, "/", nil)
	rec := test.NewResponseRecorder()
	e.ServeHTTP(rq, rec)

	assert.Empty(t, rec.Header().Get(echo.ContentEncoding))
	b, err := ioutil.ReadAll(rec.Body)
	if assert.NoError(t, err) {
		assert.Equal(t, "error", string(b))
	}
}

func TestGzipReverseProxy(t *testing.T) {
	backend := echo.New()
	backend.Get("/", echo.HandlerFunc(func(c echo.Context) error {
		return c.String(200, "Hello, world!")
	}))
	go backend.Run(standard.New(":2000"))

	backendURL, err := url.Parse("http://127.0.0.1:2000")
	assert.NoError(t, err)

	frontend := echo.New()
	frontend.Use(Gzip())
	frontend.Get("/", standard.WrapHandler(httputil.NewSingleHostReverseProxy(backendURL)))
	go frontend.Run(standard.New(":4000"))

	time.Sleep(time.Millisecond * 10)

	resp, err := http.Get("http://127.0.0.1:4000")
	assert.NoError(t, err)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, "Hello, world!", string(body))
}
