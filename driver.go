package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	ftpserver "github.com/fclairamb/ftpserverlib"
	"github.com/spf13/afero"
	"github.com/studio-b12/gowebdav"
)

type NextCloudDriver struct {
	nextcloudURL       string
	enableTLS          bool
	debug              bool
	insecureSkipVerify bool
}

func (d *NextCloudDriver) GetSettings() (*ftpserver.Settings, error) {
	port := os.Getenv("FTP_PORT")
	if port == "" {
		port = "2121"
	}

	// Default passive port range
	minPort := 30000
	maxPort := 30100

	// Override from environment variables if provided
	if pasvMinPort := os.Getenv("PASV_MIN_PORT"); pasvMinPort != "" {
		fmt.Sscanf(pasvMinPort, "%d", &minPort)
	}
	if pasvMaxPort := os.Getenv("PASV_MAX_PORT"); pasvMaxPort != "" {
		fmt.Sscanf(pasvMaxPort, "%d", &maxPort)
	}

	log.Printf("Passive port range: %d-%d", minPort, maxPort)

	settings := &ftpserver.Settings{
		ListenAddr:              "0.0.0.0:" + port,
		ActiveTransferPortNon20: true,
		PassiveTransferPortRange: &ftpserver.PortRange{
			Start: minPort,
			End:   maxPort,
		},
	}

	if d.enableTLS {
		settings.TLSRequired = ftpserver.ImplicitEncryption
	}

	return settings, nil
}

func (d *NextCloudDriver) ClientConnected(cc ftpserver.ClientContext) (string, error) {
	log.Printf("[%s] Client connected", cc.RemoteAddr())
	return "Welcome to NextCloud FTP Gateway", nil
}

func (d *NextCloudDriver) ClientDisconnected(cc ftpserver.ClientContext) {
	log.Printf("[%s] Client disconnected", cc.RemoteAddr())
}

func (d *NextCloudDriver) AuthUser(cc ftpserver.ClientContext, user, pass string) (ftpserver.ClientDriver, error) {
	remoteAddr := cc.RemoteAddr().String()
	
	sessionLogger := log.New(os.Stdout, "", log.LstdFlags)
	sessionLogger.Printf("[%s] Authentication attempt: user=%s", remoteAddr, user)

	baseURL, err := url.Parse(d.nextcloudURL)
	if err != nil {
		return nil, fmt.Errorf("invalid nextcloud URL: %w", err)
	}
	
	baseURL.Path = path.Join(baseURL.Path, "public.php/dav/files", url.PathEscape(pass))
	webdavURL := baseURL.String()
	
	client := gowebdav.NewClient(webdavURL, user, pass)

	// Configure HTTP transport
	var transport http.RoundTripper = http.DefaultTransport
	
	if d.insecureSkipVerify {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		transport = t
		log.Printf("[%s] Warning: TLS certificate verification disabled", remoteAddr)
	}

	if d.debug {
		client.SetTransport(&debugTransport{Transport: transport})
	} else {
		client.SetTransport(transport)
	}

	return &NextCloudFS{
		client:     client,
		username:   user,
		logger:     sessionLogger,
		remoteAddr: remoteAddr,
	}, nil
}

func (d *NextCloudDriver) GetTLSConfig() (*tls.Config, error) {
	if !d.enableTLS {
		return nil, nil
	}

	cert, err := generateSelfSignedCert()
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

type NextCloudFS struct {
	client     *gowebdav.Client
	username   string
	logger     *log.Logger
	remoteAddr string
}

func (fs *NextCloudFS) Name() string {
	return "NextCloudFS"
}

func (fs *NextCloudFS) Stat(name string) (os.FileInfo, error) {
	fs.logger.Printf("[%s] Stat: %s", fs.remoteAddr, name)

	info, err := fs.client.Stat(name)
	if err != nil {
		return nil, err
	}

	return &nextcloudFileInfo{
		name:    strings.TrimSuffix(path.Base(info.Name()), "/"),
		size:    info.Size(),
		mode:    info.Mode(),
		modTime: info.ModTime(),
		isDir:   info.IsDir(),
	}, nil
}

func (fs *NextCloudFS) ReadDir(name string) ([]os.FileInfo, error) {
	fs.logger.Printf("[%s] ReadDir: %s", fs.remoteAddr, name)

	files, err := fs.client.ReadDir(name)
	if err != nil {
		return nil, err
	}

	fileInfos := make([]os.FileInfo, 0, len(files))
	for _, file := range files {
		fileInfos = append(fileInfos, &nextcloudFileInfo{
			name:    strings.TrimSuffix(path.Base(file.Name()), "/"),
			size:    file.Size(),
			mode:    file.Mode(),
			modTime: file.ModTime(),
			isDir:   file.IsDir(),
		})
	}

	return fileInfos, nil
}

func (fs *NextCloudFS) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	fs.logger.Printf("[%s] OpenFile: %s (flag=%d)", fs.remoteAddr, name, flag)

	if flag&os.O_RDONLY != 0 {
		stream, err := fs.client.ReadStream(name)
		if err != nil {
			return nil, err
		}
		return &readOnlyFile{stream: stream, name: name}, nil
	}

	if flag&os.O_WRONLY != 0 || flag&os.O_CREATE != 0 {
		return &writeOnlyFile{fs: fs, name: name}, nil
	}

	return nil, errors.New("unsupported file open mode")
}

func (fs *NextCloudFS) Mkdir(name string, perm os.FileMode) error {
	fs.logger.Printf("[%s] Mkdir: %s", fs.remoteAddr, name)
	return fs.client.Mkdir(name, perm)
}

func (fs *NextCloudFS) MkdirAll(path string, perm os.FileMode) error {
	fs.logger.Printf("[%s] MkdirAll: %s", fs.remoteAddr, path)
	return fs.client.MkdirAll(path, perm)
}

func (fs *NextCloudFS) Remove(name string) error {
	fs.logger.Printf("[%s] Remove: %s", fs.remoteAddr, name)
	return fs.client.Remove(name)
}

func (fs *NextCloudFS) RemoveAll(path string) error {
	fs.logger.Printf("[%s] RemoveAll: %s", fs.remoteAddr, path)
	return fs.client.RemoveAll(path)
}

func (fs *NextCloudFS) Rename(oldname, newname string) error {
	fs.logger.Printf("[%s] Rename: %s -> %s", fs.remoteAddr, oldname, newname)
	return fs.client.Rename(oldname, newname, false)
}

func (fs *NextCloudFS) Chmod(name string, mode os.FileMode) error {
	return errors.New("chmod not supported")
}

func (fs *NextCloudFS) Chtimes(name string, atime, mtime time.Time) error {
	return errors.New("chtimes not supported")
}

func (fs *NextCloudFS) Chown(name string, uid, gid int) error {
	return errors.New("chown not supported")
}

func (fs *NextCloudFS) Create(name string) (afero.File, error) {
	fs.logger.Printf("[%s] Create: %s", fs.remoteAddr, name)
	return fs.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
}

func (fs *NextCloudFS) Open(name string) (afero.File, error) {
	fs.logger.Printf("[%s] Open: %s", fs.remoteAddr, name)
	return fs.OpenFile(name, os.O_RDONLY, 0)
}

func (fs *NextCloudFS) GetHandle(name string, flag int, offset int64) (afero.File, error) {
	return fs.OpenFile(name, flag, 0644)
}

type readOnlyFile struct {
	stream io.ReadCloser
	name   string
}

func (f *readOnlyFile) Read(p []byte) (int, error) {
	return f.stream.Read(p)
}

func (f *readOnlyFile) Write(p []byte) (int, error) {
	return 0, errors.New("write not supported on read-only file")
}

func (f *readOnlyFile) Seek(offset int64, whence int) (int64, error) {
	return 0, errors.New("seek not supported")
}

func (f *readOnlyFile) ReadAt(p []byte, off int64) (int, error) {
	return 0, errors.New("readat not supported on stream")
}

func (f *readOnlyFile) WriteAt(p []byte, off int64) (int, error) {
	return 0, errors.New("writeat not supported on read-only file")
}

func (f *readOnlyFile) Close() error {
	return f.stream.Close()
}

func (f *readOnlyFile) Name() string {
	return f.name
}

func (f *readOnlyFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, errors.New("readdir not supported on file")
}

func (f *readOnlyFile) Readdirnames(n int) ([]string, error) {
	return nil, errors.New("readdirnames not supported on file")
}

func (f *readOnlyFile) Stat() (os.FileInfo, error) {
	return nil, errors.New("stat not supported on stream")
}

func (f *readOnlyFile) Sync() error {
	return nil
}

func (f *readOnlyFile) Truncate(size int64) error {
	return errors.New("truncate not supported on read-only file")
}

func (f *readOnlyFile) WriteString(s string) (int, error) {
	return 0, errors.New("write not supported on read-only file")
}

type writeOnlyFile struct {
	fs     *NextCloudFS
	name   string
	buffer []byte
}

func (f *writeOnlyFile) Read(p []byte) (int, error) {
	return 0, errors.New("read not supported on write-only file")
}

func (f *writeOnlyFile) Write(p []byte) (int, error) {
	f.buffer = append(f.buffer, p...)
	return len(p), nil
}

func (f *writeOnlyFile) Seek(offset int64, whence int) (int64, error) {
	return 0, errors.New("seek not supported")
}

func (f *writeOnlyFile) ReadAt(p []byte, off int64) (int, error) {
	return 0, errors.New("readat not supported on write-only file")
}

func (f *writeOnlyFile) WriteAt(p []byte, off int64) (int, error) {
	return 0, errors.New("writeat not supported")
}

func (f *writeOnlyFile) Close() error {
	if len(f.buffer) == 0 {
		return nil
	}

	err := f.fs.client.Write(f.name, f.buffer, 0644)
	if err != nil {
		return fmt.Errorf("failed to write to nextcloud: %w", err)
	}

	f.fs.logger.Printf("[%s] Successfully wrote file %s (%d bytes)", f.fs.remoteAddr, f.name, len(f.buffer))
	return nil
}

type debugTransport struct {
	Transport http.RoundTripper
}

func (t *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqDump, err := httputil.DumpRequestOut(req, true)
	if err == nil {
		log.Printf("DEBUG: WebDAV Request:\n%s", string(reqDump))
	}

	resp, err := t.Transport.RoundTrip(req)
	if err != nil {
		log.Printf("DEBUG: Request failed: %v", err)
		return nil, err
	}

	respDump, _ := httputil.DumpResponse(resp, true)
	log.Printf("DEBUG: WebDAV Response:\n%s", string(respDump))

	return resp, err
}

func (f *writeOnlyFile) Name() string {
	return f.name
}

func (f *writeOnlyFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, errors.New("readdir not supported on file")
}

func (f *writeOnlyFile) Readdirnames(n int) ([]string, error) {
	return nil, errors.New("readdirnames not supported on file")
}

func (f *writeOnlyFile) Stat() (os.FileInfo, error) {
	return nil, errors.New("stat not supported on stream")
}

func (f *writeOnlyFile) Sync() error {
	return nil
}

func (f *writeOnlyFile) Truncate(size int64) error {
	return errors.New("truncate not supported")
}

func (f *writeOnlyFile) WriteString(s string) (int, error) {
	return f.Write([]byte(s))
}
