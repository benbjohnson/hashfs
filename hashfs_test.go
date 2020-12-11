package hashfs_test

import (
	"embed"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/benbjohnson/hashfs"
)

//go:embed testdata
var fsys embed.FS

func TestFormatName(t *testing.T) {
	t.Run("WithExt", func(t *testing.T) {
		if got, want := hashfs.FormatName("x.txt", "0000"), "x-0000.txt"; got != want {
			t.Fatalf("FormatName=%q, want %q", got, want)
		}
	})
	t.Run("NoExt", func(t *testing.T) {
		if got, want := hashfs.FormatName("x", "0000"), "x-0000"; got != want {
			t.Fatalf("FormatName=%q, want %q", got, want)
		}
	})
	t.Run("MultipleExt", func(t *testing.T) {
		if got, want := hashfs.FormatName("x.tar.gz", "0000"), "x-0000.tar.gz"; got != want {
			t.Fatalf("FormatName=%q, want %q", got, want)
		}
	})
	t.Run("NoHash", func(t *testing.T) {
		if got, want := hashfs.FormatName("x", ""), "x"; got != want {
			t.Fatalf("FormatName=%q, want %q", got, want)
		}
	})
	t.Run("NoFilename", func(t *testing.T) {
		if got, want := hashfs.FormatName("", "0000"), ""; got != want {
			t.Fatalf("FormatName=%q, want %q", got, want)
		}
	})
}

func TestParseName(t *testing.T) {
	t.Run("WithExt", func(t *testing.T) {
		base, hash := hashfs.ParseName("baz-b633a587c652d02386c4f16f8c6f6aab7352d97f16367c3c40576214372dd628.html")
		if got, want := base, "baz.html"; got != want {
			t.Fatalf("base=%q, want %q", got, want)
		} else if got, want := hash, "b633a587c652d02386c4f16f8c6f6aab7352d97f16367c3c40576214372dd628"; got != want {
			t.Fatalf("base=%q, want %q", got, want)
		}
	})

	t.Run("NoExt", func(t *testing.T) {
		base, hash := hashfs.ParseName("baz-b633a587c652d02386c4f16f8c6f6aab7352d97f16367c3c40576214372dd628")
		if got, want := base, "baz"; got != want {
			t.Fatalf("base=%q, want %q", got, want)
		} else if got, want := hash, "b633a587c652d02386c4f16f8c6f6aab7352d97f16367c3c40576214372dd628"; got != want {
			t.Fatalf("base=%q, want %q", got, want)
		}
	})

	t.Run("MultipleExt", func(t *testing.T) {
		base, hash := hashfs.ParseName("baz-b633a587c652d02386c4f16f8c6f6aab7352d97f16367c3c40576214372dd628.tar.gz")
		if got, want := base, "baz.tar.gz"; got != want {
			t.Fatalf("base=%q, want %q", got, want)
		} else if got, want := hash, "b633a587c652d02386c4f16f8c6f6aab7352d97f16367c3c40576214372dd628"; got != want {
			t.Fatalf("base=%q, want %q", got, want)
		}
	})

	t.Run("ShortHash", func(t *testing.T) {
		base, hash := hashfs.ParseName("baz-b633a587c652d02386c4f16f8c6f6aab7352d97f16367c3c40576214372dd62.tar.gz")
		if got, want := base, "baz-b633a587c652d02386c4f16f8c6f6aab7352d97f16367c3c40576214372dd62.tar.gz"; got != want {
			t.Fatalf("base=%q, want %q", got, want)
		} else if got, want := hash, ""; got != want {
			t.Fatalf("base=%q, want %q", got, want)
		}
	})

	t.Run("WithDir", func(t *testing.T) {
		base, hash := hashfs.ParseName("testdata/baz-b633a587c652d02386c4f16f8c6f6aab7352d97f16367c3c40576214372dd628.tar.gz")
		if got, want := base, "testdata/baz.tar.gz"; got != want {
			t.Fatalf("base=%q, want %q", got, want)
		} else if got, want := hash, "b633a587c652d02386c4f16f8c6f6aab7352d97f16367c3c40576214372dd628"; got != want {
			t.Fatalf("base=%q, want %q", got, want)
		}
	})

	t.Run("Blank", func(t *testing.T) {
		base, hash := hashfs.ParseName("")
		if got, want := base, ""; got != want {
			t.Fatalf("base=%q, want %q", got, want)
		} else if got, want := hash, ""; got != want {
			t.Fatalf("base=%q, want %q", got, want)
		}
	})
}

func TestFS_Name(t *testing.T) {
	t.Run("Exists", func(t *testing.T) {
		f := hashfs.NewFS(fsys)
		if got, want := f.HashName("testdata/baz.html"), `testdata/baz-b633a587c652d02386c4f16f8c6f6aab7352d97f16367c3c40576214372dd628.html`; got != want {
			t.Fatalf("HashName()=%q, want %q", got, want)
		}

		// Fetch a second time to pull from cache.
		if got, want := f.HashName("testdata/baz.html"), `testdata/baz-b633a587c652d02386c4f16f8c6f6aab7352d97f16367c3c40576214372dd628.html`; got != want {
			t.Fatalf("HashName()=%q, want %q", got, want)
		}
	})

	t.Run("NotExists", func(t *testing.T) {
		if got, want := hashfs.NewFS(fsys).HashName("testdata/foobar"), `testdata/foobar`; got != want {
			t.Fatalf("HashName()=%q, want %q", got, want)
		}
	})
}

func TestFS_Open(t *testing.T) {
	t.Run("ExistsNoHash", func(t *testing.T) {
		if buf, err := fs.ReadFile(hashfs.NewFS(fsys), "testdata/baz.html"); err != nil {
			t.Fatal(err)
		} else if got, want := string(buf), `<html></html>`; got != want {
			t.Fatalf("ReadFile()=%q, want %q", got, want)
		}
	})

	t.Run("ExistsWithHash", func(t *testing.T) {
		f := hashfs.NewFS(fsys)
		if buf, err := fs.ReadFile(f, "testdata/baz-b633a587c652d02386c4f16f8c6f6aab7352d97f16367c3c40576214372dd628.html"); err != nil {
			t.Fatal(err)
		} else if got, want := string(buf), `<html></html>`; got != want {
			t.Fatalf("ReadFile()=%q, want %q", got, want)
		}

		// Read again to fetch from the cache.
		if buf, err := fs.ReadFile(f, "testdata/baz-b633a587c652d02386c4f16f8c6f6aab7352d97f16367c3c40576214372dd628.html"); err != nil {
			t.Fatal(err)
		} else if got, want := string(buf), `<html></html>`; got != want {
			t.Fatalf("ReadFile()=%q, want %q", got, want)
		}
	})

	t.Run("ExistsWithMismatchHash", func(t *testing.T) {
		if _, err := fs.ReadFile(hashfs.NewFS(fsys), "testdata/baz-0000000000000000000000000000000000000000000000000000000000000000.html"); !os.IsNotExist(err) {
			t.Fatal("expected not exists")
		}
	})

	t.Run("NotExists", func(t *testing.T) {
		if _, err := fs.ReadFile(hashfs.NewFS(fsys), "nosuchfile"); !os.IsNotExist(err) {
			t.Fatal("expected not exists")
		}
	})
}

func TestFileServer(t *testing.T) {
	t.Run("NoHash", func(t *testing.T) {
		r, _ := http.NewRequest("GET", "testdata/baz.html", nil)
		w := httptest.NewRecorder()
		h := hashfs.FileServer(fsys)
		h.ServeHTTP(w, r)

		hdr := w.Result().Header
		if got, want := w.Code, 200; got != want {
			t.Fatalf("code=%v, want %v", got, want)
		} else if got, want := hdr.Get("Cache-Control"), ``; got != want {
			t.Fatalf("cache-control=%v, want %v", got, want)
		} else if got, want := hdr.Get("Content-Type"), `text/html; charset=utf-8`; got != want {
			t.Fatalf("content-type=%v, want %v", got, want)
		} else if got, want := hdr.Get("Content-Length"), `13`; got != want {
			t.Fatalf("content-length=%v, want %v", got, want)
		} else if got, want := w.Body.String(), `<html></html>`; got != want {
			t.Fatalf("body=%q, want %q", got, want)
		}
	})

	t.Run("WithHash", func(t *testing.T) {
		r, _ := http.NewRequest("GET", "testdata/baz-b633a587c652d02386c4f16f8c6f6aab7352d97f16367c3c40576214372dd628.html", nil)
		w := httptest.NewRecorder()
		h := hashfs.FileServer(fsys)
		h.ServeHTTP(w, r)

		hdr := w.Result().Header
		if got, want := w.Code, 200; got != want {
			t.Fatalf("code=%v, want %v", got, want)
		} else if got, want := hdr.Get("Cache-Control"), `public, max-age=31536000`; got != want {
			t.Fatalf("cache-control=%v, want %v", got, want)
		} else if got, want := hdr.Get("Content-Type"), `text/html; charset=utf-8`; got != want {
			t.Fatalf("content-type=%v, want %v", got, want)
		} else if got, want := hdr.Get("Content-Length"), `13`; got != want {
			t.Fatalf("content-length=%v, want %v", got, want)
		} else if got, want := w.Body.String(), `<html></html>`; got != want {
			t.Fatalf("body=%q, want %q", got, want)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		r, _ := http.NewRequest("GET", "nosuchfile", nil)
		w := httptest.NewRecorder()
		h := hashfs.FileServer(fsys)
		h.ServeHTTP(w, r)

		if got, want := w.Code, 404; got != want {
			t.Fatalf("code=%v, want %v", got, want)
		} else if got, want := w.Body.String(), "404 page not found\n"; got != want {
			t.Fatalf("body=%q, want %q", got, want)
		}
	})

	t.Run("Dir", func(t *testing.T) {
		r, _ := http.NewRequest("GET", "testdata", nil)
		w := httptest.NewRecorder()
		h := hashfs.FileServer(fsys)
		h.ServeHTTP(w, r)

		if got, want := w.Code, 403; got != want {
			t.Fatalf("code=%v, want %v", got, want)
		} else if got, want := w.Body.String(), "403 Forbidden\n"; got != want {
			t.Fatalf("body=%q, want %q", got, want)
		}
	})

	t.Run("Root", func(t *testing.T) {
		r, _ := http.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		h := hashfs.FileServer(fsys)
		h.ServeHTTP(w, r)

		if got, want := w.Code, 403; got != want {
			t.Fatalf("code=%v, want %v", got, want)
		} else if got, want := w.Body.String(), "403 Forbidden\n"; got != want {
			t.Fatalf("body=%q, want %q", got, want)
		}
	})
}
