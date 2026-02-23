package grpc

import (
	"net"

	"google.golang.org/grpc"
	grpcHealth "google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

func NewServer() *grpc.Server {
	server := grpc.NewServer()
	healthServer := grpcHealth.NewServer()
	healthv1.RegisterHealthServer(server, healthServer)
	return server
}

func Listen(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}
