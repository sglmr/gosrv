package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	addr      = flag.String("addr", ":8080", "HTTP service address")
	directory = flag.String("dir", "./", "Directory to serve")
	clients   = make(map[chan bool]bool)
	clientsMu sync.Mutex
)

// FileInfo stores the file path and its last modification time
type FileInfo struct {
	Path    string
	ModTime time.Time
}

// EventSource handler for live reload
func handleEventSource(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE (Server-Sent Events)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create a channel for this client
	messageChan := make(chan bool)

	// Register new client
	clientsMu.Lock()
	clients[messageChan] = true
	clientsMu.Unlock()

	// Remove client when disconnected
	defer func() {
		clientsMu.Lock()
		delete(clients, messageChan)
		close(messageChan)
		clientsMu.Unlock()
	}()

	// Set a timeout for the connection
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Keep connection alive
	notify := w.(http.CloseNotifier).CloseNotify()

	// Send initial connection message
	fmt.Fprintf(w, "event: connected\ndata: %d\n\n", time.Now().Unix())
	flusher.Flush()

	// Wait for messages or connection close
	for {
		select {
		case <-notify:
			return
		case <-messageChan:
			fmt.Fprintf(w, "event: reload\ndata: %d\n\n", time.Now().Unix())
			flusher.Flush()
		case <-time.After(25 * time.Second):
			// Send keep-alive comment to keep connection open
			fmt.Fprintf(w, ": keepalive %d\n\n", time.Now().Unix())
			flusher.Flush()
		}
	}
}

// injectLiveReload modifies HTML files to include the EventSource client
func injectLiveReload(w http.ResponseWriter, r *http.Request, path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Create JavaScript for live reload using EventSource
	liveReloadScript := `
<script>
    (function() {
        const evtSource = new EventSource('/events');
        
        evtSource.addEventListener('connected', function(e) {
            console.log('Live reload connected');
        });
        
        evtSource.addEventListener('reload', function(e) {
            console.log('Live reload triggered:', e.data);
            window.location.reload();
        });
        
        evtSource.onerror = function() {
            console.log('Live reload disconnected');
            evtSource.close();
            // Try to reconnect every 2 seconds
            setTimeout(function() {
                window.location.reload();
            }, 2000);
        };
    })();
</script>
`

	// Insert the script before the closing </body> tag
	htmlStr := string(content)
	if strings.Contains(htmlStr, "</body>") {
		htmlStr = strings.Replace(htmlStr, "</body>", liveReloadScript+"</body>", 1)
	} else if strings.Contains(htmlStr, "</html>") {
		// If no body tag, try html tag
		htmlStr = strings.Replace(htmlStr, "</html>", liveReloadScript+"</html>", 1)
	} else {
		// If no body or html tag, append to the end
		htmlStr = htmlStr + liveReloadScript
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(htmlStr))
}

// FileServerWithLiveReload serves files with live reload injection
func FileServerWithLiveReload(dir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Convert path to filepath
		path := filepath.Join(dir, filepath.Clean(r.URL.Path))

		// Handle root path
		if r.URL.Path == "/" {
			// Try to find index.html
			indexPath := filepath.Join(dir, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				path = indexPath
			}
		}

		// Check if file exists
		info, err := os.Stat(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		// If it's a directory, look for index.html
		if info.IsDir() {
			indexPath := filepath.Join(path, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				path = indexPath
			} else {
				// Try to serve directory listing
				http.FileServer(http.Dir(dir)).ServeHTTP(w, r)
				return
			}
		}

		// Inject live reload for HTML files
		if strings.HasSuffix(strings.ToLower(path), ".html") {
			injectLiveReload(w, r, path)
			return
		}

		// Serve other files directly
		http.FileServer(http.Dir(dir)).ServeHTTP(w, r)
	})
}

// notifyClients sends a reload message to all EventSource clients
func notifyClients() {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	for client := range clients {
		// Non-blocking send
		select {
		case client <- true:
			// Successfully sent
		default:
			// Channel full or closed, will be cleaned up on next cycle
		}
	}
}

// scanDirectory scans a directory and returns a map of files with their modification times
func scanDirectory(dir string) (map[string]time.Time, error) {
	fileMap := make(map[string]time.Time)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files and directories and node_modules
		basename := filepath.Base(path)
		if basename[0] == '.' || basename == "node_modules" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Store the file's path and modification time
		fileMap[path] = info.ModTime()
		return nil
	})

	return fileMap, err
}

// watchDirectoryForChanges periodically scans the directory for changes
func watchDirectoryForChanges(dir string, interval time.Duration) {
	// Initial scan of the directory
	prevFiles, err := scanDirectory(dir)
	if err != nil {
		log.Fatal("Failed to scan directory:", err)
	}

	// Periodically scan the directory for changes
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		currentFiles, err := scanDirectory(dir)
		if err != nil {
			log.Println("Error scanning directory:", err)
			continue
		}

		changes := false

		// Check for new or modified files
		for path, modTime := range currentFiles {
			prevModTime, exists := prevFiles[path]
			if !exists || modTime.After(prevModTime) {
				log.Println("File changed:", path)
				changes = true
			}
		}

		// Check for deleted files
		for path := range prevFiles {
			if _, exists := currentFiles[path]; !exists {
				log.Println("File deleted:", path)
				changes = true
			}
		}

		// Update the previous files map
		prevFiles = currentFiles

		// Notify clients if there were changes
		if changes {
			notifyClients()
		}
	}
}

func main() {
	flag.Parse()

	// Resolve absolute path
	absDir, err := filepath.Abs(*directory)
	if err != nil {
		log.Fatal("Failed to resolve directory path:", err)
	}

	// Start file watcher in a goroutine
	// Poll for changes every 500ms
	go watchDirectoryForChanges(absDir, 500*time.Millisecond)

	// Setup handlers
	http.Handle("/", FileServerWithLiveReload(absDir))
	http.HandleFunc("/events", handleEventSource)

	// Start the server
	log.Printf("Starting development server at http://localhost%s serving directory %s", *addr, absDir)
	log.Printf("Press Ctrl+C to stop")

	err = http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
