package interceptor

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// AuthInterceptor returns a new unary server interceptor that checks for authorization headers.
func AuthInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		values := metadata.ValueFromIncomingContext(ctx, "authorization")

		if len(values) == 0 || values[0] != "Bearer secret-token" {
			logger.Warn("Unauthenticated request blocked")
			return nil, status.Errorf(codes.Unauthenticated, "missing or invalid authorization token")
		}

		return handler(ctx, req)
	}
}
