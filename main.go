package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/rs/cors"
)

const gracefulShutdownTimeout = 10 * time.Second

type Scores struct {
	DevOps int
	Fe     int
	Be     int
}

type Company struct {
	Scores Scores
	Name   string
}

var list_of_companies = []Company{
	{
		Scores: Scores{1, 1, 1},
		Name:   "EARLY LTD",
	},
	{
		Scores: Scores{3, 3, 3},
		Name:   "INT LTD",
	},
	{
		Scores: Scores{1, 1, 1},
		Name:   "EXPERT LTD",
	},
}

func main() {
	go heartbeat()

	r := http.NewServeMux()
	r.HandleFunc("/status/ready", readinessCheckEndpoint)

	// Something for the example frontend to hit
	r.HandleFunc("/status/about", func(w http.ResponseWriter, r *http.Request) {

		response, err := json.Marshal(map[string]string{"podName": os.Getenv("POD_NAME")})
		if err == nil {

			w.Header().Set("Content-Type", "application/json")
			w.Write(response)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	r.HandleFunc("/analysis", func(w http.ResponseWriter, r *http.Request) {
		var scores Scores

		err := json.NewDecoder(r.Body).Decode(&scores)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		fmt.Println(list_of_companies)
		// Call db ( Mimic with json for now )
		// Cosine Similarity
		// Respond with company info
	})

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
	})

	// Change this to a more strict CORS policy
	handler := cors.Default().Handler(r)
	//NOTE: ACTIVATE BELOW FOR STAGING/PROD
	// serverAddress := fmt.Sprintf("0.0.0.0:%s", os.Getenv("SERVER_PORT"))
	//NOTE: ACTIVATE BELOW FOR DEVELOPMENT
	serverAddress := fmt.Sprintf("localhost:%s", "8080")
	server := &http.Server{Addr: serverAddress, Handler: handler}

	// Watch for signals to handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Run the server in a goroutine
	go func() {
		log.Printf("Serving at http://%s/", serverAddress)
		err := server.ListenAndServe()
		if err != http.ErrServerClosed {
			log.Fatalf("Fatal error while serving HTTP: %v\n", err)
			close(stop)
		}
	}()

	// Block while reading from the channel until we receive a signal
	sig := <-stop
	log.Printf("Received signal %s, starting graceful shutdown", sig)

	// Give connections some time to drain
	ctx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()
	err := server.Shutdown(ctx)
	if err != nil {
		log.Fatalf("Error during shutdown, client requests have been terminated: %v\n", err)
	} else {
		log.Println("Graceful shutdown complete")
	}
}

// Simple health check endpoint
func readinessCheckEndpoint(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, strconv.Quote("OK"))
}

func heartbeat() {
	for range time.Tick(4 * time.Second) {
		fh, err := os.Create("/tmp/service-alive")
		if err != nil {
			log.Println("Unable to write file for liveness check!")
		} else {
			fh.Close()
		}
	}
}
