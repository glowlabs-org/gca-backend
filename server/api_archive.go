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

// ArchiveHandler provides the POST /archive api endpoint. It returns
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

	buffer := new(bytes.Buffer)
	znw := zip.NewWriter(buffer)

	// File ordering matters here. We are not locking the files, and relying on the file writes being
	// append only while happening in a single write. We must archive the files in reverse order from
	// their dependency order.

	for _, name := range PublicFiles {
		err := gcas.addFile(name, znw)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error zipping file %v: %v", name, err), http.StatusInternalServerError)
			return
		}
	}

	// Add the pseudo file server.pubkey last, as the other files are signed by it.
	if err := gcas.addPubKeyFile("server.pubkey", znw); err != nil {
		http.Error(w, fmt.Sprintf("Error zipping server.pubkey: %v", err), http.StatusInternalServerError)
		return
	}

	// Add a README
	if err := gcas.addReadmeFile(znw); err != nil {
		http.Error(w, fmt.Sprintf("Error adding README: %v", err), http.StatusInternalServerError)
		return
	}

	if err := znw.Close(); err != nil {
		if _, err := w.Write(buffer.Bytes()); err != nil {
			http.Error(w, "Failed to close zip writer", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/zip")

	if _, err := w.Write(buffer.Bytes()); err != nil {
		http.Error(w, "Failed to write archive response", http.StatusInternalServerError)
	}
}

// Add a file to a zip archive, copying all the data in the file.
func (gcas *GCAServer) addFile(name string, zipw *zip.Writer) error {
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

	header, err := zip.FileInfoHeader(fileInfo)
	if err != nil {
		return err
	}

	header.Method = zip.Deflate

	writer, err := zipw.CreateHeader(header)
	if err != nil {
		return err
	}

	// File contains only public data so copy it all.
	if _, err := io.Copy(writer, file); err != nil {
		return err
	}

	return nil
}

// Add a pseudo-file "server.pubkey" to a zip archive,
// copying only the public key part of the data file "server.keys".
func (gcas *GCAServer) addPubKeyFile(name string, znw *zip.Writer) error {

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

	writer, err := znw.CreateHeader(&zip.FileHeader{
		Name:     "server.pubkey",
		Method:   zip.Deflate,
		Modified: fileInfo.ModTime(),
	})
	if err != nil {
		return err
	}

	if _, err := io.Copy(writer, reader); err != nil {
		return err
	}

	return nil
}

// Add a README pseudo-file to a zip archive.
func (gcas *GCAServer) addReadmeFile(znw *zip.Writer) error {

	var buf bytes.Buffer

	buf.WriteString(ReadmeContents)

	writer, err := znw.CreateHeader(&zip.FileHeader{
		Name:     "README",
		Method:   zip.Deflate,
		Modified: time.Now(),
	})
	if err != nil {
		return err
	}

	// Copy all the file contents
	if _, err := io.Copy(writer, &buf); err != nil {
		return err
	}
	return nil
}
