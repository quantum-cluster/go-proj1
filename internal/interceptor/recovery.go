package interceptor

import (
	"context"
	"log/slog"
	"runtime/debug"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryRecoveryInterceptor returns a new unary server interceptor that recovers from panics.
func UnaryRecoveryInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ any, err error) {
		defer func() {
			if r := recover(); r != nil {
				// Log the panic with stack trace
				logger.Error("Captured panic in unary handler",
					slog.Any("panic", r),
					slog.String("stack", string(debug.Stack())),
					slog.String("method", info.FullMethod),
				)
				err = status.Errorf(codes.Internal, "Internal server error: %v", r)
			}
		}()

		return handler(ctx, req)
	}
}
