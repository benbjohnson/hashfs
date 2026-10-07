package hashfs_test

import (
	"embed"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"

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

func TestParseName_InvalidHashSuffix(t *testing.T) {
	hash := strings.Repeat("a", 64)
	for _, suffix := range []string{"0", "x", "-", "-other"} {
		for _, ext := range []string{"", ".txt", ".tar.gz"} {
			name := "assets/file-" + hash + suffix + ext
			t.Run(suffix+ext, func(t *testing.T) {
				base, hash := hashfs.ParseName(name)
				if base != name || hash != "" {
					t.Fatalf("ParseName(%q)=(%q, %q), want (%q, %q)", name, base, hash, name, "")
				}
			})
		}
	}
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
		} else if got, want := hdr.Get("ETag"), ""; got != want {
			t.Fatalf("etag=%v, want %v", got, want)
		} else if got, want := w.Body.String(), `<html></html>`; got != want {
			t.Fatalf("body=%q, want %q", got, want)
		}
	})

	t.Run("WithHash", func(t *testing.T) {
		r, _ := http.NewRequest("GET", "testdata/baz-b633a587c652d02386c4f16f8c6f6aab7352d97f16367c3c40576214372dd628.html", nil)
		h := hashfs.FileServer(fsys)
		hash := "\"b633a587c652d02386c4f16f8c6f6aab7352d97f16367c3c40576214372dd628\""

		for i := 0; i < 2; i++ {
			w := httptest.NewRecorder()
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
			} else if got, want := hdr.Get("ETag"), hash; got != want {
				t.Fatalf("etag=%v, want %v", got, want)
			} else if got, want := w.Body.String(), `<html></html>`; got != want {
				t.Fatalf("body=%q, want %q", got, want)
			}
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

func TestFileServer_UnverifiedHash(t *testing.T) {
	hash := strings.Repeat("a", 64)
	for _, name := range []string{
		"file-" + hash + "0.txt",
		"file-" + hash + "x.txt",
		"file-" + hash + "-.txt",
		"file-" + hash + ".txt",
		"missing-" + hash + ".txt",
	} {
		t.Run(name, func(t *testing.T) {
			fsys := fstest.MapFS{
				"file.txt": &fstest.MapFile{Data: []byte("base contents")},
				name:       &fstest.MapFile{Data: []byte("literal filename contents")},
			}
			h := hashfs.FileServer(fsys)
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				t.Run(method, func(t *testing.T) {
					w := httptest.NewRecorder()
					h.ServeHTTP(w, httptest.NewRequest(method, "/"+name, nil))
					if got := w.Code; got != http.StatusOK {
						t.Fatalf("code=%d, want %d", got, http.StatusOK)
					}
					for _, field := range []string{"Cache-Control", "ETag"} {
						if got := w.Header().Get(field); got != "" {
							t.Errorf("%s=%q, want empty for literal filename", field, got)
						}
					}
					want := "literal filename contents"
					if method == http.MethodHead {
						want = ""
					}
					if got := w.Body.String(); got != want {
						t.Errorf("body=%q, want %q", got, want)
					}
				})
			}
		})
	}
}
