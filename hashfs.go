package hashfs

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Ensure file system implements interface.
var _ fs.FS = (*FS)(nil)

// FS represents an fs.FS file system that can optionally use content addressable
// hashes in the filename. This allows the caller to aggressively cache the
// data since the filename will change if the data changes.
type FS struct {
	fsys fs.FS

	mu sync.RWMutex
	m  map[string]string    // lookup (path to hash path)
	r  map[string][2]string // reverse lookup (hash path to path)
}

func NewFS(fsys fs.FS) *FS {
	return &FS{
		fsys: fsys,
		m:    make(map[string]string),
		r:    make(map[string][2]string),
	}
}

// Open returns a reference to the named file.
// If name is a hash name then the underlying file is used.
func (fsys *FS) Open(name string) (fs.File, error) {
	f, _, err := fsys.open(name)
	return f, err
}

func (fsys *FS) open(name string) (_ fs.File, hash string, err error) {
	// Parse filename to see if it contains a hash.
	// If so, check if hash name matches.
	base, hash := fsys.ParseName(name)
	if hash != "" && fsys.HashName(base) == name {
		name = base
	}

	f, err := fsys.fsys.Open(name)
	return f, hash, err
}

// HashName returns the hash name for a path, if exists.
// Otherwise returns the original path.
func (fsys *FS) HashName(name string) string {
	// Lookup cached formatted name, if exists.
	fsys.mu.RLock()
	if s := fsys.m[name]; s != "" {
		fsys.mu.RUnlock()
		return s
	}
	fsys.mu.RUnlock()

	// Read file contents. Return original filename if we receive an error.
	buf, err := fs.ReadFile(fsys.fsys, name)
	if err != nil {
		return name
	}

	// Compute hash and build filename.
	hash := sha256.Sum256(buf)
	hashhex := hex.EncodeToString(hash[:])
	hashname := FormatName(name, hashhex)

	// Store in lookups.
	fsys.mu.Lock()
	fsys.m[name] = hashname
	fsys.r[hashname] = [2]string{name, hashhex}
	fsys.mu.Unlock()

	return hashname
}

// FormatName returns a hash name that inserts hash before the filename's
// extension. If no extension exists on filename then the hash is appended.
// Returns blank string the original filename if hash is blank. Returns a blank
// string if the filename is blank.
func FormatName(filename, hash string) string {
	if filename == "" {
		return ""
	} else if hash == "" {
		return filename
	}

	dir, base := path.Split(filename)
	if i := strings.Index(base, "."); i != -1 {
		return path.Join(dir, fmt.Sprintf("%s-%s%s", base[:i], hash, base[i:]))
	}
	return path.Join(dir, fmt.Sprintf("%s-%s", base, hash))
}

// ParseName splits formatted hash filename into its base & hash components.
func (fsys *FS) ParseName(filename string) (base, hash string) {
	fsys.mu.RLock()
	defer fsys.mu.RUnlock()

	if hashed, ok := fsys.r[filename]; ok {
		return hashed[0], hashed[1]
	}

	return ParseName(filename)
}

// ParseName splits formatted hash filename into its base & hash components.
func ParseName(filename string) (base, hash string) {
	if filename == "" {
		return "", ""
	}

	dir, base := path.Split(filename)

	// Extract pre-hash & extension.
	pre, ext := base, ""
	if i := strings.Index(base, "."); i != -1 {
		pre = base[:i]
		ext = base[i:]
	}

	// If prehash doesn't contain the hash, then exit.
	if !hashSuffixRegex.MatchString(pre) {
		return filename, ""
	}

	return path.Join(dir, pre[:len(pre)-65]+ext), pre[len(pre)-64:]
}

var hashSuffixRegex = regexp.MustCompile(`-[0-9a-f]{64}`)

// FileServer returns an http.Handler for serving FS files. It provides a
// simplified implementation of http.FileServer which is used to aggressively
// cache files on the client since the file hash is in the filename.
//
// Because FileServer is focused on small known path files, several features
// of http.FileServer have been removed including canonicalizing directories,
// defaulting index.html pages, precondition checks, & content range headers.
func FileServer(fsys fs.FS) http.Handler {
	hfsys, ok := fsys.(*FS)
	if !ok {
		hfsys = NewFS(fsys)
	}
	return &fsHandler{fsys: hfsys}
}

type fsHandler struct {
	fsys *FS
}

func (h *fsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Clean up filename based on URL path.
	filename := r.URL.Path
	if filename == "/" {
		filename = "."
	} else {
		filename = strings.TrimPrefix(filename, "/")
	}
	filename = path.Clean(filename)

	// Read file from attached file system.
	f, hash, err := h.fsys.open(filename)
	if errors.Is(err, fs.ErrNotExist) {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	// Fetch file info. Disallow directories from being displayed.
	fi, err := f.Stat()
	if err != nil {
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		return
	} else if fi.IsDir() {
		http.Error(w, "403 Forbidden", http.StatusForbidden)
		return
	}

	// Cache the file aggressively if the file contains a hash.
	if hash != "" {
		w.Header().Set("Cache-Control", `public, max-age=31536000`)
		w.Header().Set("ETag", "\""+hash+"\"")
	}

	// Flush header and write content.
	switch f := f.(type) {
	case io.ReadSeeker:
		http.ServeContent(w, r, filename, fi.ModTime(), f.(io.ReadSeeker))
	default:
		// Set content length.
		w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))

		// Flush header and write content.
		w.WriteHeader(http.StatusOK)
		if r.Method != "HEAD" {
			io.Copy(w, f)
		}
	}
}
