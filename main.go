package main

import (
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	port      = "8080"   // Default port
	shareDir  = "./file" // Default sharing directory
	baseURL   string
	startTime time.Time
	fileList  []string
	mu        sync.Mutex
)

func main() {
	startTime = time.Now()

	if len(os.Args) > 1 {
		port = os.Args[1]
	}
	if len(os.Args) > 2 {
		shareDir = os.Args[2]
	}

	absPath, err := filepath.Abs(shareDir)
	if err != nil {
		log.Fatal("Error getting absolute path:", err)
	}
	shareDir = absPath

	fileList, err = listFiles(shareDir)
	if err != nil {
		log.Fatal("Error listing files:", err)
	}

	baseURL = fmt.Sprintf("http://%s:%s/", getLocalIP(), port)

	fmt.Println("Sharing files from:", shareDir)
	fmt.Println("Server started at:", baseURL)
	fmt.Println("Use Ctrl+C to stop.")

	http.HandleFunc("/", fileListHandler)
	http.HandleFunc("/download/", downloadHandler)

	err = http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

func fileListHandler(w http.ResponseWriter, r *http.Request) {
	files, err := listFiles(shareDir)
	if err != nil {
		http.Error(w, "Error listing files", http.StatusInternalServerError)
		return
	}

	data := struct {
		Files  []string
		Uptime string
	}{
		Files:  files,
		Uptime: time.Since(startTime).String(),
	}

	// Template with modern UI
	tmpl := template.Must(template.New("index").Funcs(template.FuncMap{
		"isImage": func(fileName string) bool {
			ext := strings.ToLower(filepath.Ext(fileName))
			return ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp"
		},
		"isVideo": func(fileName string) bool {
			ext := strings.ToLower(filepath.Ext(fileName))
			return ext == ".mp4" || ext == ".webm" || ext == ".ogg"
		},
		"fileIcon": func(fileName string) string {
			ext := strings.ToLower(filepath.Ext(fileName))
			icons := map[string]string{
				".pdf":  "📄",
				".txt":  "📝",
				".zip":  "📦",
				".rar":  "📦",
				".docx": "📃",
				".xlsx": "📊",
				".pptx": "📽",
				".mp3":  "🎵",
				".wav":  "🎶",
			}
			if icon, found := icons[ext]; found {
				return icon
			}
			return "📁" // Default icon
		},
	}).Parse(`
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>File Sharing</title>
  <style>
    body { font-family: Arial, sans-serif; background-color: #0a192f; color: #ffffff; margin: 0; padding: 20px; }
    .container { max-width: 800px; margin: 0 auto; }
    h1 { color: #64ffda; text-align: center; }
    .file-list { list-style: none; padding: 0; }
    .file-item { background-color: #112240; padding: 15px; border-radius: 8px; margin-bottom: 10px; display: flex; align-items: center; gap: 15px; }
    .file-item img, .file-item video { max-width: 100px; max-height: 100px; border-radius: 5px; }
    .file-icon { width: 50px; height: 50px; display: flex; align-items: center; justify-content: center; background-color: #233554; border-radius: 5px; font-size: 20px; }
    .file-name { flex-grow: 1; color: #ffffff; text-decoration: none; }
    .file-name:hover { text-decoration: underline; }
    .download-btn { background-color: #64ffda; color: #0a192f; border: none; padding: 8px 12px; border-radius: 5px; cursor: pointer; text-decoration: none; }
    .download-btn:hover { background-color: #52e3c2; }
    .uptime { text-align: center; margin-top: 20px; color: #8892b0; }
  </style>
</head>
<body>
  <div class="container">
    <h1>Shared Files</h1>
    <ul class="file-list">
      {{range .Files}}
      <li class="file-item">
        {{if isImage .}}
        <img src="/download/{{.}}" alt="{{.}}">
        {{else if isVideo .}}
        <video controls muted>
          <source src="/download/{{.}}" type="video/mp4">
          Your browser does not support the video tag.
        </video>
        {{else}}
        <div class="file-icon">{{fileIcon .}}</div>
        {{end}}
        <span class="file-name">{{.}}</span>
        <a href="/download/{{.}}" class="download-btn" download>Download</a>
      </li>
      {{end}}
    </ul>
    <div class="uptime">Server started {{.Uptime}} ago</div>
  </div>
</body>
</html>
`))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
	}
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/download/")
	filepath := filepath.Join(shareDir, filename)

	mu.Lock()
	defer mu.Unlock()

	if !fileExists(filepath) {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, filepath)
}

func listFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			relPath, _ := filepath.Rel(dir, path)
			files = append(files, relPath)
		}
		return nil
	})
	return files, err
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	return err == nil && !info.IsDir()
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "localhost"
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "localhost"
}
