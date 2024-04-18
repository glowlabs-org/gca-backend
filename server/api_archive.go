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

	"github.com/glowlabs-org/gca-backend/glow"
)

var (
	ApiArchiveRateLimiter *glow.RateLimiter // Provide a rate limiter for this API.
)

func init() {
	fmt.Printf("init rate limiter limit %v rate %v\n", apiArchiveLimit, apiArchiveRate)
	ApiArchiveRateLimiter = glow.NewRateLimiter(apiArchiveLimit, apiArchiveRate)
}

// ArchiveHandler provides the POST /archive api endpoint. It returns
// uninterpreted file data intended to be used by an external service
// to produce an archive.
func (gcas *GCAServer) ArchiveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.ContentLength != 0 {
		http.Error(w, "Request should not have a body", http.StatusBadRequest)
		return
	}

	if !ApiArchiveRateLimiter.Allow() {
		http.Error(w, "Too many requests", http.StatusTooManyRequests)
		return
	}

	buffer := new(bytes.Buffer)
	zipw := zip.NewWriter(buffer)

	files := []string{"server.keys", "gcaTempPubKey.dat", "gcaPubKey.dat", "equipment-authorizations.dat", "allDeviceStats.dat", "equipment-reports.dat"}

	for _, name := range files {
		err := gcas.addFile(name, zipw)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error zipping file %v: %v", name, err), http.StatusInternalServerError)
			return
		}
	}

	if err := zipw.Close(); err != nil {
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

func (gcas *GCAServer) addFile(name string, zipw *zip.Writer) error {
	// To avoid races with any components writing to the file, lock up here.
	gcas.mu.Lock()
	defer gcas.mu.Unlock()

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

	if name == "server.keys" {
		// Special case, this file contains private data, so we need to sanitize it.

		data, err := io.ReadAll(file)
		if err != nil {
			return err
		}

		// Check if this file is the correct size
		if len(data) != 96 {
			return fmt.Errorf("%v has incorrect size", name)
		}

		writer.Write(data[:32])        // Copy the public keys
		writer.Write(make([]byte, 64)) // Write zeroes to the rest of the file
	} else {
		// File contains only public data so copy it all.
		if _, err := io.Copy(writer, file); err != nil {
			return err
		}
	}
	return nil
}
