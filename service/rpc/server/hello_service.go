package server

import (
	"context"

	"plaud-emails/external/helloservice"
)

// HelloServiceServer 实现HelloService的gRPC服务
type HelloServiceServer struct {
	helloservice.UnimplementedHelloServiceServer
}

// NewHelloServiceServer 创建新的HelloService服务器
func NewHelloServiceServer() *HelloServiceServer {
	return &HelloServiceServer{}
}

func (p *HelloServiceServer) SayHello(ctx context.Context, req *helloservice.SayHelloRequest) (*helloservice.SayHelloResponse, error) {
	return &helloservice.SayHelloResponse{
		Msg: "Hello, " + req.Msg,
	}, nil
}
