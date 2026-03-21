package interceptor

import (
	"context"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func AuthInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	values := metadata.ValueFromIncomingContext(ctx, "authorization")

	if len(values) == 0 || values[0] != "Bearer secret-token" {
		log.Println("Unauthenticated")
		return nil, status.Errorf(codes.Unauthenticated, "...")
	}

	return handler(ctx, req)
}
