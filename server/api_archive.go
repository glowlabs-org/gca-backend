package server

// This is a low-level archiver which zips and sends GCA server persistent file
// data using a POST command. This is intended to be used by an external service
// to provide archival snapshots of raw data in the system. Only public data is
// provided, with private keys zeroed before sending.

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// The contents of a README file which will be inserted into the archive.
const ReadmeContents = `This archive contains uninterpreted server files.
These files all contain publicly available information
and additional work is needed to stand up a new server
using them.
`

// Zip archive helper.
type zipArchiveWriter struct {
	buffer *bytes.Buffer
	znw    *zip.Writer
}

// Create a new zip archive writer.
func NewArchive() *zipArchiveWriter {
	buf := new(bytes.Buffer)
	return &zipArchiveWriter{
		buffer: buf,
		znw:    zip.NewWriter(buf),
	}
}

// Close the archive writer and return a buffer to its contents.
func (arc *zipArchiveWriter) Close() (*bytes.Buffer, error) {
	if err := arc.znw.Close(); err != nil {
		return nil, err
	}
	return arc.buffer, nil
}

func (arc *zipArchiveWriter) AddFile(reader io.Reader, name string, modTime time.Time) error {
	writer, err := arc.znw.CreateHeader(&zip.FileHeader{
		Name:     name,
		Method:   zip.Deflate,
		Modified: modTime,
	})
	if err != nil {
		return err
	}
	if _, err := io.Copy(writer, reader); err != nil {
		return err
	}
	return nil
}

// ArchiveHandler provides the GET /archive api endpoint. It returns
// uninterpreted file data intended to be used by an external service
// to produce an archive.
func (gcas *GCAServer) ArchiveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.ContentLength != 0 {
		http.Error(w, "Request should not have a body", http.StatusBadRequest)
		return
	}
	if !gcas.ApiArchiveRateLimiter.Allow() {
		http.Error(w, fmt.Sprintf("Too many requests, this server allows %v every %v", apiArchiveLimit, apiArchiveRate), http.StatusTooManyRequests)
		return
	}
	arc := NewArchive()

	// File ordering matters here. We are not locking the files, and relying on the file writes being
	// append-only while happening in a single write. We must archive the files in reverse order from
	// their dependency order.
	for _, pf := range PublicFiles {
		err := gcas.addFile(pf, arc)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error zipping file %v: %v", pf, err), http.StatusInternalServerError)
			return
		}
	}
	// Add the pseudo file server.pubkey last, as the other files are signed by it.
	if err := gcas.addPubKeyFile("server.pubkey", arc); err != nil {
		http.Error(w, fmt.Sprintf("Error zipping server.pubkey: %v", err), http.StatusInternalServerError)
		return
	}
	if err := gcas.addReadmeFile(arc); err != nil {
		http.Error(w, fmt.Sprintf("Error adding README: %v", err), http.StatusInternalServerError)
		return
	}

	// Close archive and write out the response
	buf, err := arc.Close()
	if err != nil {
		http.Error(w, fmt.Sprintf("%v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	if _, err := w.Write(buf.Bytes()); err != nil {
		http.Error(w, "Failed to write archive response", http.StatusInternalServerError)
		return
	}
}

// Add a file to an archive, copying all the data in the file.
func (gcas *GCAServer) addFile(name string, arc *zipArchiveWriter) error {
	path := filepath.Join(gcas.baseDir, name)
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}
	if err := arc.AddFile(file, name, fileInfo.ModTime()); err != nil {
		return err
	}
	return nil
}

// Add a pseudo-file "server.pubkey" to an archive, copying only the
// public key part of the data file "server.keys".
func (gcas *GCAServer) addPubKeyFile(name string, arc *zipArchiveWriter) error {
	path := filepath.Join(gcas.baseDir, "server.keys")
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	// Read only the public key from the file, using the
	// server API.
	pub, _, err := gcas.loadGCAServerKeys()
	if err != nil {
		return err
	}
	reader := bytes.NewReader(pub[:])

	// Read the first 32 bytes from the server.keys file
	// and verify that we have the same public key data.
	buf := make([]byte, 32)
	if _, err := file.Read(buf); err != nil {
		return err
	}
	if !bytes.Equal(pub[:], buf) {
		return fmt.Errorf("public key values differ")
	}

	// It's safe to copy the data now.
	if err := arc.AddFile(reader, name, fileInfo.ModTime()); err != nil {
		return err
	}
	return nil
}

// Add a README pseudo-file to an archive.
func (gcas *GCAServer) addReadmeFile(arc *zipArchiveWriter) error {
	var buf bytes.Buffer
	buf.WriteString(ReadmeContents)
	if err := arc.AddFile(&buf, "README", time.Now()); err != nil {
		return err
	}
	return nil
}
