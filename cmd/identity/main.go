package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	pb "github.com/quantum-cluster/go-proj1/protos/v1"
	"google.golang.org/grpc"
)

type identityServer struct {
	pb.UnimplementedIdentityServer
}

func main() {
	grpcServer := grpc.NewServer()
	pb.RegisterIdentityServer(grpcServer, &identityServer{})

	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Default().Fatalf("Can't listen: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup

	wg.Go(func() {
		log.Default().Println("Starting server on port :50051")

		if err := grpcServer.Serve(listener); err != nil {
			log.Default().Printf("Terminating server: %v", err)
		}
	})

	<-ctx.Done()
	log.Default().Println("Context expired. Exiting gracefully...")
	grpcServer.GracefulStop()
	wg.Wait()
	log.Default().Println("Process complete")
}
