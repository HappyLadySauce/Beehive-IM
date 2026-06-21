package main

import (
	"flag"
	"fmt"

	"github.com/HappyLadySauce/Beehive-IM/services/notification/internal/config"
	"github.com/HappyLadySauce/Beehive-IM/services/notification/internal/server"
	"github.com/HappyLadySauce/Beehive-IM/services/notification/internal/svc"
	"github.com/HappyLadySauce/Beehive-IM/services/notification/pb"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/beehiveim.notification.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	ctx := svc.NewServiceContext(c)
	defer ctx.Close()

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterNotificationServiceServer(grpcServer, server.NewNotificationServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
