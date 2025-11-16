package grpc

import (
	"context"
	"fmt"
	"log"
	"net"

	pb "github.com/fileserver/transfer/api/proto"
	"github.com/fileserver/transfer/internal/config"
	"github.com/fileserver/transfer/internal/progress"
	"github.com/fileserver/transfer/internal/transfer"
	"google.golang.org/grpc"
)

// Server represents the gRPC server
type Server struct {
	pb.UnimplementedFileTransferServer
	config   *config.Config
	receiver *transfer.Receiver
	tracker  *progress.Tracker
	grpcSrv  *grpc.Server
}

// NewServer creates a new gRPC server
func NewServer(cfg *config.Config, tracker *progress.Tracker) *Server {
	return &Server{
		config:   cfg,
		receiver: transfer.NewReceiver(cfg.AllowedDir, tracker),
		tracker:  tracker,
	}
}

// Start starts the gRPC server
func (s *Server) Start() error {
	lis, err := net.Listen("tcp", s.config.GRPCListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.grpcSrv = grpc.NewServer(
		grpc.MaxRecvMsgSize(10 * 1024 * 1024), // 10MB max message size
		grpc.MaxSendMsgSize(10 * 1024 * 1024),
	)
	pb.RegisterFileTransferServer(s.grpcSrv, s)

	log.Printf("Starting gRPC server on %s", s.config.GRPCListenAddr)

	go func() {
		if err := s.grpcSrv.Serve(lis); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	return nil
}

// Stop stops the gRPC server
func (s *Server) Stop() {
	if s.grpcSrv != nil {
		s.grpcSrv.GracefulStop()
	}
}

// TransferFile handles file transfer requests
func (s *Server) TransferFile(stream pb.FileTransfer_TransferFileServer) error {
	log.Println("Receiving file transfer request")
	return s.receiver.ReceiveFile(stream)
}

// GetPeerInfo returns peer information for startup verification
func (s *Server) GetPeerInfo(ctx context.Context, req *pb.PeerInfoRequest) (*pb.PeerInfoResponse, error) {
	return &pb.PeerInfoResponse{
		TargetServer:   s.config.TargetServer,
		GrpcListenAddr: s.config.GRPCListenAddr,
	}, nil
}

// HealthCheck performs a health check
func (s *Server) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{
		Healthy: true,
		Message: "Server is healthy",
	}, nil
}
