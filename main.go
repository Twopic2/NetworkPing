package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type Server struct {
	Name        string    `json:"name"`
	Address     string    `json:"address"`
	Status      string    `json:"status"`
	Latency     int64     `json:"latency"`
	LastChecked time.Time `json:"lastChecked"`
}

var servers []Server
var serverMutex sync.RWMutex

func pingServers(server *Server) {
	start := time.Now()

	cmd := exec.Command("ping", "-c 1", "-w 2", server.Address)
	output, err := cmd.CombinedOutput()

	timeMilli := time.Since(start).Milliseconds()
	server.LastChecked = time.Now()

	if err != nil {
		server.Status = "Gone"
		server.Latency = 0
		return
	}

	if strings.Contains(string(output), "1 received") {
		server.Status = "online"
		server.Latency = timeMilli
	} else {
		server.Status = "offline"
		server.Latency = 0
	}

}

func pingAllServers() {

	serverMutex.Lock()
	defer serverMutex.Unlock()

	var wg sync.WaitGroup
	for i := range servers {
		wg.Add(1)
		go func(a int) {
			defer wg.Done()
			pingServers(&servers[a])
		}(i)
	}
	wg.Wait()

}

func serverList() {

	servers = append(servers, Server{
		Name:    "Proxmox",
		Address: "192.168.0.18",
		Status:  "unknown",
	})

	servers = append(servers, Server{
		Name:    "NextCloud",
		Address: "192.168.0.122",
		Status:  "unknown",
	})

	servers = append(servers, Server{
		Name:    "Website",
		Address: "192.168.0.68",
		Status:  "unknown",
	})

	servers = append(servers, Server{
		Name:    "Digital Ocean VPS",
		Address: "146.190.153.87",
		Status:  "unknown",
	})

}

func getServersHandler(w http.ResponseWriter, r *http.Request) {
	serverMutex.RLock()
	defer serverMutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(servers)
}

func pingServerNow(w http.ResponseWriter, r *http.Request) {
	go pingAllServers()
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"pinging"}`))
}

func serverJSON() {
	serverMutex.RLock()
	defer serverMutex.RUnlock()

	file, err := os.Create("servers.json")

	if err != nil {
		log.Fatal()
		return
	}

	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(servers)
	if err != nil {
		log.Fatal()
	}

}

func webHanlder() {

	http.HandleFunc("/api/servers", getServersHandler)
	http.HandleFunc("/api/ping", pingServerNow)

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

}

func main() {

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	serverList()

	webHanlder()

	go func() {

		pingAllServers()

		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			<-ticker.C
			pingAllServers()
			serverJSON()
		}

	}()

	log.Printf("Server starting on port %s", port)
	log.Printf("Web UI available at http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))

}
