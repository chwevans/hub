package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	_ "embed"
)

//go:embed upload.html
var uploadHTML string

// Get preferred outbound ip of this machine
func GetOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}

const _uploadKey = "upload"

func main() {
	if len(os.Args) == 2 && os.Args[1] == _uploadKey {
		http.HandleFunc("/", buildUploadFile())
		http.HandleFunc("/upload", buildUploadFileHandler(_uploadKey))
	} else if len(os.Args) == 2 {
		if _, err := os.Stat(os.Args[1]); !os.IsNotExist(err) {
			http.HandleFunc("/", buildServeFile(os.Args[1]))
		}
	} else {
		response := strings.Join(os.Args[1:], " ")
		http.HandleFunc("/", buildServeString(response))
	}

	inbound := GetOutboundIP()

	fmt.Printf("Serving on %s:8003\n", inbound)
	http.ListenAndServe("0.0.0.0:8003", nil)
}

func buildServeString(response string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(response))
	}
}

func buildServeFile(fileName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f, err := os.Open(fileName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer f.Close()

		fileName := filepath.Base(fileName)
		w.Header().Set("Content-Disposition", "attachment; filename=\""+fileName+"\"")
		w.Header().Set("Content-Type", "application/octet-stream")
		io.Copy(w, f)
	}
}

func buildUploadFile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Return html that will trigger a file upload
		w.Write([]byte(uploadHTML))
	}
}

func buildUploadFileHandler(destinationDirectory string) http.HandlerFunc {
	// ensure destinationDirectory exists
	if _, err := os.Stat(destinationDirectory); os.IsNotExist(err) {
		os.Mkdir(destinationDirectory, 0755)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Handle file upload
		if err := receiveFile(r, destinationDirectory); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	}
}

const _maxSize = 32 << 20

func receiveFile(r *http.Request, destinationDirectory string) error {
	r.ParseMultipartForm(_maxSize) // limit your max input length!
	// in your case file would be fileupload
	file, header, err := r.FormFile("file")
	if err != nil {
		return err
	}
	defer file.Close()
	fmt.Printf("File name %s\n", header.Filename)

	baseName := filepath.Base(header.Filename)
	fmt.Println(baseName)

	destination := filepath.Join(destinationDirectory, baseName)
	fmt.Println(destination)

	f, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer f.Close()

	io.Copy(f, file)
	return nil
}
