// Command server boots the PII redaction engine's HTTP API.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kaizakin/PII-redactor/internal/api"
	"github.com/kaizakin/PII-redactor/internal/config"
	"github.com/kaizakin/PII-redactor/internal/detector"
	"github.com/kaizakin/PII-redactor/internal/grpcclient"
	"github.com/kaizakin/PII-redactor/internal/processor"
)

// buildDetectors assembles the active set of Detector strategies. This is
// the one place a new PII type needs to be wired in: implement the
// detector.Detector interface and append an instance here — nothing else
// in the pipeline changes.
func buildDetectors(cfg config.Config) []detector.Detector {
	// nlpClient talks to the Python NLP worker for unstructured PII
	// (names, companies, physical addresses). Until cfg.NLPWorkerAddr
	// points at a running worker, NoOpClient keeps those detectors
	// registered but inert, so the engine still runs correctly with
	// structured detection alone.
	var nlpClient grpcclient.NLPClient = grpcclient.NoOpClient{}

	return []detector.Detector{
		detector.NewEmailDetector(),
		detector.NewPhoneDetector(cfg.PhoneRegion),
		detector.NewSSNDetector(),
		detector.NewCreditCardDetector(),
		detector.NewIPDetector(),
		detector.NewDOBDetector(),
		detector.NewNLPDetector(nlpClient, detector.TypeName, "PERSON"),
		detector.NewNLPDetector(nlpClient, detector.TypeCompany, "ORG"),
		detector.NewNLPDetector(nlpClient, detector.TypeAddress, "ADDRESS"),
	}
}

func main() {
	cfg := config.Load()

	handler := api.NewHandler(buildDetectors(cfg), processor.DefaultGenerators())
	router := api.NewRouter(handler)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("pii-redactor listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}
}
