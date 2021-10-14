package web

import (
	"embed"
	"errors"
	"io"
	"mime"
	"net/http"
	"path"
	"path/filepath"
)

//go:embed build/*
var build embed.FS

func AssetHandler(w http.ResponseWriter, r *http.Request) {
	err := asset(build, "build", r.URL.Path, w)
	if err == nil {
		return
	}
	err = asset(build, "build", "index.html", w)
	if err != nil {
		panic(err)
	}
}

func asset(fs embed.FS, prefix, requestedPath string, w http.ResponseWriter) error {
	f, err := fs.Open(path.Join(prefix, requestedPath))
	if err != nil {
		return err
	}
	defer f.Close()

	stat, _ := f.Stat()
	if stat.IsDir() {
		return errors.New("unexpected dir read")
	}

	contentType := mime.TypeByExtension(filepath.Ext(requestedPath))
	w.Header().Set("Content-Type", contentType)
	_, err = io.Copy(w, f)
	return err
}
