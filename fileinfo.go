package main

import (
	"os"
	"time"
)

type nextcloudFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (fi *nextcloudFileInfo) Name() string {
	return fi.name
}

func (fi *nextcloudFileInfo) Size() int64 {
	return fi.size
}

func (fi *nextcloudFileInfo) Mode() os.FileMode {
	return fi.mode
}

func (fi *nextcloudFileInfo) ModTime() time.Time {
	return fi.modTime
}

func (fi *nextcloudFileInfo) IsDir() bool {
	return fi.isDir
}

func (fi *nextcloudFileInfo) Sys() any {
	return nil
}

type listerat []os.FileInfo

func (l listerat) ListAt(f []os.FileInfo, offset int64) (int, error) {
	if offset >= int64(len(l)) {
		return 0, nil
	}

	n := copy(f, l[offset:])
	return n, nil
}
