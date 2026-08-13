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

// buildNLPClient connects to the Python NLP worker when cfg.NLPWorkerAddr
// is configured. Until it is, NoOpClient keeps the unstructured detectors
// registered but inert, so the engine still runs correctly with
// structured detection alone.
func buildNLPClient(cfg config.Config) grpcclient.NLPClient {
	if cfg.NLPWorkerAddr == "" {
		log.Print("NLP_WORKER_ADDR not set: unstructured PII (names, companies, addresses) will not be detected")
		return grpcclient.NoOpClient{}
	}
	client, err := grpcclient.Dial(cfg.NLPWorkerAddr)
	if err != nil {
		log.Fatalf("failed to connect to NLP worker at %s: %v", cfg.NLPWorkerAddr, err)
	}
	log.Printf("connected to NLP worker at %s", cfg.NLPWorkerAddr)
	return client
}

// buildDetectors assembles the active set of Detector strategies. This is
// the one place a new PII type needs to be wired in: implement the
// detector.Detector interface and append an instance here — nothing else
// in the pipeline changes.
func buildDetectors(cfg config.Config, nlpClient grpcclient.NLPClient) []detector.Detector {
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

	nlpClient := buildNLPClient(cfg)
	defer nlpClient.Close()

	handler := api.NewHandler(buildDetectors(cfg, nlpClient), processor.DefaultGenerators())
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
