package file

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

var (
	ErrFileNotFound = errors.New("file not found")
)

type FileTree struct {
	Path     string
	Name     string
	Size     string
	IsDir    bool
	IsBadDir bool
	Children []*FileTree
}

func (f *FileTree) FindMatch(fpath string) (*FileTree, error) {
	if fpath == "/" {
		return f, nil
	}
	parts := strings.Split(strings.Trim(fpath, "/"), "/")
	for _, part := range parts {
		found := false
		for _, child := range f.Children {
			if child.Name == part {
				f = child
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("%w: %s", ErrFileNotFound, fpath)
		}
	}
	return f, nil
}

func GetFileTree(errWriter io.Writer, fpath string) (*FileTree, error) {
	fstat, err := os.Stat(fpath)
	if err != nil {
		return nil, err
	}
	root := &FileTree{
		Path:  fpath,
		Name:  fstat.Name(),
		IsDir: fstat.IsDir(),
	}
	queue := []*FileTree{root}
	for len(queue) > 0 {
		f := queue[0]
		queue = queue[1:]
		if f.IsDir {
			f.Size = " - "
			entries, err := os.ReadDir(f.Path)
			if err != nil {
				f.IsBadDir = true
				fmt.Fprintln(errWriter, err)
				continue
			}
			for _, entry := range entries {
				finfo, err := entry.Info()
				if err != nil {
					fmt.Fprintln(errWriter, err)
					continue
				}
				child := &FileTree{
					Path:  path.Join(f.Path, entry.Name()),
					Name:  entry.Name(),
					IsDir: finfo.IsDir(),
				}
				f.Children = append(f.Children, child)
				queue = append(queue, child)
			}
		} else {
			f.Size = FormatSize(fstat.Size())
		}
	}
	return root, nil
}

func FormatSize(fsize int64) string {
	var (
		unit   string
		factor int64
	)
	if factor = 1024 * 1024 * 2014; fsize > factor {
		unit = "GB"
	} else if factor = 1024 * 1024; fsize > factor {
		unit = "MB"
	} else if factor = 1024; fsize > factor {
		unit = "KB"
	} else {
		unit = "B"
		factor = 1
	}
	return fmt.Sprintf("%.2f%s", float64(fsize)/float64(factor), unit)
}
