package grpcclient

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/kaizakin/PII-redactor/proto/gen/redactor"
)

// GRPCClient is the production NLPClient: it talks to the Python NLP
// worker over gRPC using the stubs generated from proto/redactor.proto.
type GRPCClient struct {
	conn   *grpc.ClientConn
	client pb.NLPServiceClient
}

// Dial connects to the NLP worker at addr (e.g. "localhost:50051"). The
// underlying grpc.ClientConn connects lazily, so Dial returning without
// error does not guarantee the worker is reachable yet — a connection
// failure surfaces on the first Analyze call instead.
func Dial(addr string) (*GRPCClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpcclient: dial %q: %w", addr, err)
	}
	return &GRPCClient{conn: conn, client: pb.NewNLPServiceClient(conn)}, nil
}

// Analyze sends text to the NLP worker and translates its response into
// the engine's own Entity type, keeping the generated protobuf types out
// of every other package's imports.
func (c *GRPCClient) Analyze(ctx context.Context, text string) ([]Entity, error) {
	start := time.Now()
	resp, err := c.client.Analyze(ctx, &pb.AnalyzeRequest{Text: text})
	if err != nil {
		log.Printf("grpcclient: analyze call failed after %s: %v", time.Since(start), err)
		return nil, fmt.Errorf("grpcclient: analyze: %w", err)
	}
	entities := make([]Entity, 0, len(resp.GetEntities()))
	for _, e := range resp.GetEntities() {
		entities = append(entities, Entity{
			Type:       e.GetType(),
			Start:      int(e.GetStart()),
			End:        int(e.GetEnd()),
			Text:       e.GetText(),
			Confidence: e.GetConfidence(),
		})
	}
	log.Printf("grpcclient: analyzed %d-char text, found %d entities, took %s", len(text), len(entities), time.Since(start))
	return entities, nil
}

// Close releases the underlying connection to the NLP worker.
func (c *GRPCClient) Close() error {
	return c.conn.Close()
}
