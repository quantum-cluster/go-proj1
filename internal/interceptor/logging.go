package interceptor

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryLoggingInterceptor returns a new unary server interceptor that logs request details.
func UnaryLoggingInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		st, _ := status.FromError(err)

		attrs := []any{
			slog.String("method", info.FullMethod),
			slog.String("status", st.Code().String()),
			slog.Duration("duration", duration),
		}

		if err != nil {
			attrs = append(attrs, slog.Any("error", err))
			logger.Error("RPC failed", attrs...)
		} else {
			logger.Info("RPC succeeded", attrs...)
		}

		return resp, err
	}
}
