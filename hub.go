package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"mime"
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
	if len(os.Args) == 2 {
		info, err := os.Stat(os.Args[1])
		if os.Args[1] == _uploadKey {
			fmt.Println("Waiting for files")
			http.HandleFunc("/", buildUploadFile())
			http.HandleFunc("/upload", buildUploadFileHandler(_uploadKey))
			http.HandleFunc("/upload-text", buildUploadTextHandler())
		} else if err == nil {
			fmt.Println("Serving", os.Args[1])
			if info.IsDir() {
				http.HandleFunc("/", buildServeDirectory(os.Args[1]))
			} else {
				http.HandleFunc("/", buildServeFile(os.Args[1]))
			}
		} else {
			fmt.Println("Serving string")
			response, err := getAllInput()
			if err != nil {
				log.Fatal(err)
			}
			http.HandleFunc("/", buildServeString(response))
		}
	} else {
		response, err := getAllInput()
		if err != nil {
			log.Fatal(err)
		}
		http.HandleFunc("/", buildServeString(response))
	}

	inbound := GetOutboundIP()

	fmt.Printf("Serving on %s:8003\n", inbound)
	if err := http.ListenAndServe("0.0.0.0:8003", nil); err != nil {
		log.Fatal(err)
	}
}

func getAllInput() (string, error) {
	commandLine := strings.Join(os.Args[1:], " ")

	fi, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	if fi.Size() > 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}

		commandLine += string(data)
	}
	return commandLine, nil
}

func buildServeString(response string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(response))
	}
}

func shouldServeAsDownload(extension string) bool {
	switch extension {
	case ".html", ".css", ".js":
		return false
	}
	return true
}

func buildServeFile(fileName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f, err := os.Open(fileName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer f.Close()

		extension := filepath.Ext(fileName)
		mimeType := mime.TypeByExtension(extension)

		if shouldServeAsDownload(extension) {
			downloadName := filepath.Base(fileName)
			w.Header().Set("Content-Disposition", "attachment; filename=\""+downloadName+"\"")
		}
		w.Header().Set("Content-Type", mimeType)
		io.Copy(w, f)
	}
}

func buildServeDirectory(directory string) http.HandlerFunc {
	buf, err := zipDirectory(directory)
	if err != nil {
		log.Fatal(err)
	}

	baseName := filepath.Base(directory)

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", "attachment; filename=\""+baseName+".zip\"")
		w.Header().Set("Content-Type", "application/octet-stream")
		io.Copy(w, buf)
	}
}

func buildUploadTextHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Handle text upload
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		text := r.PostFormValue("text")
		fmt.Printf("Received text: ```\n%s\n```\n", text)
		http.Redirect(w, r, "/", http.StatusSeeOther)
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
			fmt.Println("Error uploading file:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Println("File uploaded successfully")
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

const _maxSize = 32 << 20

func receiveFile(r *http.Request, destinationDirectory string) error {
	if err := r.ParseMultipartForm(_maxSize); err != nil {
		return err
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return err
	}
	defer file.Close()

	baseName := filepath.Base(header.Filename)
	destination := filepath.Join(destinationDirectory, baseName)
	fmt.Printf("Uploading (%s): %s -> %s\n", getPeer(r), header.Filename, destination)

	f, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer f.Close()

	io.Copy(f, file)
	return nil
}

func getPeer(r *http.Request) string {
	return r.RemoteAddr
}

func zipDirectory(source string) (*bytes.Buffer, error) {
	// Create a buffer to write our archive to.
	buf := new(bytes.Buffer)

	zipWriter := zip.NewWriter(buf)
	defer zipWriter.Close()

	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Determine the relative path for the file/directory within the zip
		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		// Handle directories
		if info.IsDir() {
			// Add a trailing slash for directories in the zip header
			if relPath != "." { // Don't add the root directory itself with a slash
				header, err := zip.FileInfoHeader(info)
				if err != nil {
					return err
				}
				header.Name = relPath + "/"
				_, err = zipWriter.CreateHeader(header)
				return err
			}
			return nil
		}

		// Handle files
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = relPath
		header.Method = zip.Deflate // Use Deflate compression

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		_, err = io.Copy(writer, file)
		return err
	})

	return buf, err
}
