// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package main

import (
	"flag"
	"fmt"

	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/config"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/handler"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/beehiveim.edge.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()
	server.Use(rest.ToMiddleware(handler.NewCORSMiddleware(c.Security)))

	ctx := svc.NewServiceContext(c)
	defer ctx.Close()
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
