hashfs
======

Implementation of io/fs.FS that appends SHA256 hashes to filenames to allow for
aggressive HTTP caching.

For example, given a file path of `scripts/main.js`, the `hashfs.FS`
filesystem will provide the server with a hashname of
`scripts/main-b633a..d628.js` (the hash is truncated for brevity in the example). When
this file path is requested by the client, the server can verify the hash and
return the contents with an aggressive `Cache-Control` header. The client will
cache this file for up to a year and does not need to re-request it in the
future.

Note that this library requires Go 1.16 or higher.


## Usage

To use `hashfs`, first wrap your `embed.FS` in a `hashfs.FS` filesystem:

```go
//go:embed scripts stylesheets images
var embedFS embed.FS

var fsys = hashfs.NewFS(embedFS)
```

Then attach a `hashfs.FileServer()` to your router:

```go
http.Handle("/assets/", http.StripPrefix("/assets/", hashfs.FileServer(fsys)))
```

Next, your html templating library can obtain the hashname of your file using
the `hashfs.FS.HashName()` method:

```go
func renderHTML(w io.Writer) {
	fmt.Fprintf(w, `<html>`)
	fmt.Fprintf(w, `<script src="/assets/%s"></script>`, fsys.HashName("scripts/main.js"))
	fmt.Fprintf(w, `</html>`)
}
```
