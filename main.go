package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io/ioutil"
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

type RawCompany struct {
	Id     float64 `json: "id"`
	Name   string  `json: "name"`
	Devops float64 `json: "devops"`
	Fe     float64 `json: "fe"`
	Be     float64 `json: "be"`
}

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

		// Read from JSON (TODO: Replace with db call if db is being used)
		data, err := ioutil.ReadFile("MOCK_DATA.json")
		if err != nil {
			fmt.Println(err)
			return
		}
		var rawCompanies []RawCompany
		err = json.Unmarshal(data, &rawCompanies)
		if err != nil {
			fmt.Println(err)
			return
		}

		// Read from request for Ep Scores
		var EpScores EpScores
		err = json.NewDecoder(r.Body).Decode(&EpScores)
		if err != nil {

			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Convert to Ep Scores in to float
		EpScoresFloat := []float64{
			EpScores.DevOpsScore,
			EpScores.FeScore,
			EpScores.BeScore}

		//Change Raw Company struct in to format desirable for cosine similarity
		var companies []Company
		for _, company := range rawCompanies {
			scores := make([]float64, 3)
			scores[0], scores[1], scores[2] = company.Devops, company.Fe, company.Be

			companyFixed := Company{
				Scores: scores,
				Name:   company.Name,
				CosSim: 0,
			}
			companies = append(companies, companyFixed)
		}

		// Calculate Cosine Similarity, sort them in highest to lowest value order
		var CompanyRankPayload []Company
		for _, company := range companies {
			score := company.Scores
			cos, err := Cosine(score, EpScoresFloat)
			if err != nil {
				log.Fatalf("error: %v\n", err)
			}
			cosineScore := Company{
				Scores: company.Scores,
				Name:   company.Name,
				CosSim: cos,
			}
			CompanyRankPayload = append(CompanyRankPayload, cosineScore)
		}
		sort.Slice(CompanyRankPayload, func(i, j int) bool {
			return CompanyRankPayload[i].CosSim > CompanyRankPayload[j].CosSim
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CompanyRankPayload)
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
