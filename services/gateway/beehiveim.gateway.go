package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/config"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/server"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/gateway/pb"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/beehiveim.gateway.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	ctx := svc.NewServiceContext(c)
	defer ctx.Close()

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterGatewayServiceServer(grpcServer, server.NewGatewayServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	stopOnSignal(s, ctx, c.DrainTimeoutSeconds)

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}

func stopOnSignal(server *zrpc.RpcServer, svcCtx *svc.ServiceContext, drainTimeoutSeconds int64) {
	if drainTimeoutSeconds <= 0 {
		drainTimeoutSeconds = 15
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		fmt.Println("Gateway received shutdown signal, entering draining mode...")
		drainCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := svcCtx.EnterDraining(drainCtx); err != nil {
			fmt.Printf("Gateway draining registry update failed: %v\n", err)
		}
		cancel()
		time.Sleep(time.Duration(drainTimeoutSeconds) * time.Second)
		server.Stop()
	}()
}
