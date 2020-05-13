package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"syscall"
	"time"

	"github.com/rs/cors"
)

const gracefulShutdownTimeout = 10 * time.Second

type Company struct {
	Scores []float64
	Name   string
	CosSim float64
}

type EpScores struct {
	DevOpsScore float64
	FeScore     float64
	BeScore     float64
}

var Companies = []Company{
	{
		Scores: []float64{8, 13, 14},
		Name:   "EARLY LTD",
	},
	{
		Scores: []float64{12, 17, 21},
		Name:   "INT LTD",
	},
	{
		Scores: []float64{17, 11, 24},
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
		var EpScores EpScores
		fmt.Println("@@@@", EpScores)
		err := json.NewDecoder(r.Body).Decode(&EpScores)
		if err != nil {

			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		EpScoresFloat := []float64{
			EpScores.DevOpsScore,
			EpScores.FeScore,
			EpScores.BeScore}

		var CompanyRankPayload []Company

		fmt.Println("Ep Scores are gathered... to cosine smilarlities")
		for _, company := range Companies {
			score := company.Scores
			cos, err := Cosine(score, EpScoresFloat)
			if err != nil {
				log.Fatalf("error: %v\n", err)
			}
			companyCosineScore := Company{
				Scores: company.Scores,
				Name:   company.Name,
				CosSim: cos,
			}
			CompanyRankPayload = append(CompanyRankPayload, companyCosineScore)

		}

		sort.Slice(CompanyRankPayload, func(i, j int) bool {
			return CompanyRankPayload[i].CosSim > CompanyRankPayload[j].CosSim
		})
		fmt.Println("!!!!!!!!!!!!", CompanyRankPayload)
		// cos gives result of cosine similarlities. Use index to resort the slice
		// return each name and value of company
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

func Cosine(a []float64, b []float64) (cosine float64, err error) {
	count := 0
	length_a := len(a)
	length_b := len(b)
	if length_a > length_b {
		count = length_a
	} else {
		count = length_b
	}
	sumA := 0.0
	s1 := 0.0
	s2 := 0.0
	for k := 0; k < count; k++ {
		if k >= length_a {
			s2 += math.Pow(b[k], 2)
			continue
		}
		if k >= length_b {
			s1 += math.Pow(a[k], 2)
			continue
		}
		sumA += a[k] * b[k]
		s1 += math.Pow(a[k], 2)
		s2 += math.Pow(b[k], 2)
	}
	if s1 == 0 || s2 == 0 {
		return 0.0, errors.New("Vectors should not be null (all zeros)")
	}
	return sumA / (math.Sqrt(s1) * math.Sqrt(s2)), nil
}
