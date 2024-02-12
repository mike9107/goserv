package files

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/cmgsj/goserve/pkg/templates"
	"github.com/cmgsj/goserve/pkg/util/units"
)

type Server struct {
	fs.FS
	includeDotfiles bool
	version         string
}

func NewServer(fs fs.FS, includeDotfiles bool, version string) *Server {
	return &Server{
		FS:              fs,
		includeDotfiles: includeDotfiles,
		version:         version,
	}
}

func (s *Server) ServeTemplate() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filepath := path.Clean(r.PathValue("path"))

		info, err := fs.Stat(s, filepath)
		if err != nil {
			s.sendErrorPage(w, err)
			return
		}

		if !info.IsDir() {
			err = s.sendFile(w, r, filepath)
			if err != nil {
				s.sendErrorPage(w, err)
			}
			return
		}

		entries, err := fs.ReadDir(s, filepath)
		if err != nil {
			s.sendErrorPage(w, err)
			return
		}

		err = s.sendTemplate(w, entries, filepath)
		if err != nil {
			s.sendErrorPage(w, err)
		}
	})
}

func (s *Server) ServeText() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filepath := path.Clean(r.PathValue("path"))

		info, err := fs.Stat(s, filepath)
		if err != nil {
			sendError(w, err)
			return
		}

		if !info.IsDir() {
			err = s.sendFile(w, r, filepath)
			if err != nil {
				sendError(w, err)
			}
			return
		}

		entries, err := fs.ReadDir(s, filepath)
		if err != nil {
			sendError(w, err)
			return
		}

		err = s.sendText(w, entries, filepath)
		if err != nil {
			sendError(w, err)
		}
	})
}

func (s *Server) Health() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func (s *Server) Version() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, s.version)
	})
}

func (s *Server) sendFile(w http.ResponseWriter, r *http.Request, filepath string) error {
	f, err := s.Open(filepath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(w, f)

	return err
}

func (s *Server) sendTemplate(w io.Writer, entries []fs.DirEntry, filepath string) error {
	var breadcrumbs, files []templates.File

	if filepath != "." {
		var pathPrefix string

		for _, name := range strings.Split(filepath, "/") {
			pathPrefix = path.Join(pathPrefix, name)

			breadcrumbs = append(breadcrumbs, templates.File{
				Path: pathPrefix,
				Name: name,
			})
		}

		files = append(files, templates.File{
			Path:  path.Dir(filepath),
			Name:  "..",
			IsDir: true,
		})
	}

	for _, entry := range entries {
		if !s.includeFile(entry.Name()) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		files = append(files, templates.File{
			Path:  path.Join(filepath, info.Name()),
			Name:  info.Name(),
			Size:  units.FormatSize(info.Size()),
			IsDir: info.IsDir(),
		})
	}

	sortFiles(files)

	return templates.ExecuteIndex(w, templates.Page{
		Breadcrumbs: breadcrumbs,
		Files:       files,
		Version:     s.version,
	})
}

func (s *Server) sendText(w io.Writer, entries []fs.DirEntry, filepath string) error {
	var files []templates.File

	if filepath != "." {
		files = append(files, templates.File{
			Name:  "..",
			IsDir: true,
		})
	}

	for _, entry := range entries {
		if !s.includeFile(entry.Name()) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		files = append(files, templates.File{
			Name:  info.Name(),
			Size:  units.FormatSize(info.Size()),
			IsDir: info.IsDir(),
		})
	}

	sortFiles(files)

	var buf bytes.Buffer

	for _, file := range files {
		buf.WriteString(file.Name)
		if file.IsDir {
			buf.WriteByte('/')
		} else {
			buf.WriteByte('\t')
			buf.WriteString(file.Size)
		}
		buf.WriteByte('\n')
	}

	tab := tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
	defer tab.Flush()

	_, err := io.Copy(tab, &buf)

	return err
}

func (s *Server) includeFile(name string) bool {
	return s.includeDotfiles || !strings.HasPrefix(name, ".")
}

func (s *Server) sendErrorPage(w http.ResponseWriter, err error) {
	err = templates.ExecuteIndex(w, templates.Page{
		Error:   err.Error(),
		Version: s.version,
	})
	if err != nil {
		sendError(w, err)
	}
}

func sendError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
	slog.Error("an error ocurred", "error", err)
}

func sortFiles(files []templates.File) {
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return files[i].Name < files[j].Name
	})
}
