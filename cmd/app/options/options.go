package options

import (
	"encoding/json"

	"github.com/spf13/pflag"
	"k8s.io/component-base/cli/flag"

	"github.com/HappyLadySauce/Beehive-IM/pkg/options"
)

type Options struct {
	basename        string
	InsecureServing *options.InsecureServingOptions `mapstructure:"insecure"`
	Database        *options.PostgreOptions         `mapstructure:"database"`
	Cache           *options.RedisOptions           `mapstructure:"cache"`
	RabbitMQ        *options.RabbitMQOptions        `mapstructure:"rabbitmq"`
	JWT             *options.JWTOptions             `mapstructure:"jwt"`
}

func NewOptions(basename string) *Options {
	return &Options{
		basename:        basename,
		InsecureServing: options.NewInsecureServingOptions(),
		Database:        options.NewPostgreOptions(),
		Cache:           options.NewRedisOptions(),
		RabbitMQ:        options.NewRabbitMQOptions(),
		JWT:             options.NewJWTOptions(),
	}
}

// AddFlags adds the flags to the specified FlagSet and returns the grouped flag sets.
// AddFlags 将标志注册到指定的 FlagSet，并返回分组后的 NamedFlagSets。
func (o *Options) AddFlags(fs *pflag.FlagSet) *flag.NamedFlagSets {
	nfs := &flag.NamedFlagSets{}

	// Register flags into each NamedFlagSet bucket.
	// 将各组标志注册到对应的 NamedFlagSet。
	configFS := nfs.FlagSet("Config")
	options.AddConfigFlag(configFS, o.basename)

	insecureServingFS := nfs.FlagSet("Insecure Serving")
	o.InsecureServing.AddFlags(insecureServingFS)

	databaseFS := nfs.FlagSet("Database")
	o.Database.AddFlags(databaseFS)

	cacheFS := nfs.FlagSet("Cache")
	o.Cache.AddFlags(cacheFS)

	rabbitmqFS := nfs.FlagSet("RabbitMQ")
	o.RabbitMQ.AddFlags(rabbitmqFS)

	jwtFS := nfs.FlagSet("JWT")
	o.JWT.AddFlags(jwtFS)

	// Merge all named flag sets into the root command FlagSet.
	// 将所有命名标志集合并到根命令的 FlagSet。
	for _, name := range nfs.Order {
		fs.AddFlagSet(nfs.FlagSets[name])
	}
	return nfs
}

func (o *Options) String() string {
	data, _ := json.Marshal(o)

	return string(data)
}
