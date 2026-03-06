package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/daearol/jockv2/backend/internal/cache"
	pb "github.com/daearol/jockv2/backend/internal/proto"
	"github.com/daearol/jockv2/backend/internal/server"
	"google.golang.org/grpc"
)

func main() {
	// Bind to port 0 for dynamic assignment
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Print the assigned port to stdout - Electron reads this
	port := lis.Addr().(*net.TCPAddr).Port
	fmt.Printf("JOCK_PORT=%d\n", port)
	os.Stdout.Sync()

	cacheManager := cache.NewManager(30 * time.Second)

	s := grpc.NewServer()
	pb.RegisterGitServiceServer(s, server.New(cacheManager))

	// Graceful shutdown on SIGTERM/SIGINT
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		<-sigCh
		log.Println("Shutting down gRPC server...")
		s.GracefulStop()
	}()

	log.Printf("gRPC server listening on port %d", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
