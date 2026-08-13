package grpcclient

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/kaizakin/PII-redactor/proto/gen/redactor"
)

// fakeNLPServiceServer stands in for the Python worker: it returns a fixed
// set of entities regardless of input, letting the test exercise
// GRPCClient's wire-to-Entity translation without a real NLP model.
type fakeNLPServiceServer struct {
	pb.UnimplementedNLPServiceServer
	entities   []*pb.Entity
	redactions int32
}

func (f *fakeNLPServiceServer) Analyze(ctx context.Context, req *pb.AnalyzeRequest) (*pb.AnalyzeResponse, error) {
	return &pb.AnalyzeResponse{Entities: f.entities}, nil
}

func (f *fakeNLPServiceServer) RedactImage(ctx context.Context, req *pb.RedactImageRequest) (*pb.RedactImageResponse, error) {
	// Echo the input back with a marker byte appended, so the test can
	// tell the response actually came from this handler and round-tripped
	// through the real wire format rather than being a zero value.
	return &pb.RedactImageResponse{
		ImageData:  append(append([]byte(nil), req.GetImageData()...), '!'),
		Redactions: f.redactions,
	}, nil
}

// dialFake starts fake in-process over a bufconn listener and returns a
// GRPCClient connected to it.
func dialFake(t *testing.T, fake *fakeNLPServiceServer) *GRPCClient {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	pb.RegisterNLPServiceServer(srv, fake)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return &GRPCClient{conn: conn, client: pb.NewNLPServiceClient(conn)}
}

func TestGRPCClientAnalyze(t *testing.T) {
	fake := &fakeNLPServiceServer{entities: []*pb.Entity{
		{Type: "PERSON", Start: 0, End: 11, Text: "Rashi Patil", Confidence: 0.95},
		{Type: "ORG", Start: 21, End: 28, Text: "Acme Co", Confidence: 0.8},
	}}
	client := dialFake(t, fake)

	entities, err := client.Analyze(context.Background(), "Rashi Patil works at Acme Co")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(entities) != 2 {
		t.Fatalf("expected 2 entities, got %d: %+v", len(entities), entities)
	}
	if entities[0].Type != "PERSON" || entities[0].Text != "Rashi Patil" || entities[0].Start != 0 || entities[0].End != 11 {
		t.Errorf("unexpected first entity: %+v", entities[0])
	}
	if entities[1].Type != "ORG" || entities[1].Text != "Acme Co" {
		t.Errorf("unexpected second entity: %+v", entities[1])
	}
}

func TestGRPCClientAnalyzeEmpty(t *testing.T) {
	client := dialFake(t, &fakeNLPServiceServer{})
	entities, err := client.Analyze(context.Background(), "nothing sensitive here")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(entities) != 0 {
		t.Errorf("expected no entities, got %+v", entities)
	}
}

func TestGRPCClientRedactImage(t *testing.T) {
	fake := &fakeNLPServiceServer{redactions: 3}
	client := dialFake(t, fake)

	redacted, count, err := client.RedactImage(context.Background(), []byte("fake-png-bytes"), "png")
	if err != nil {
		t.Fatalf("RedactImage: %v", err)
	}
	if string(redacted) != "fake-png-bytes!" {
		t.Errorf("unexpected redacted bytes: %q", redacted)
	}
	if count != 3 {
		t.Errorf("expected 3 redactions, got %d", count)
	}
}

func TestGRPCClientClose(t *testing.T) {
	client := dialFake(t, &fakeNLPServiceServer{})
	if err := client.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
