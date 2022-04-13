package conf

import (
	"github.com/olivere/elastic"
	"github.com/spf13/viper"
	"log"
	"os"
	"time"
)

func init() {
	// 读取yaml文件
	// config := viper.New() // 通过New加载配置则只能用其返回值获取配置
	config := viper.GetViper()          // 全局加载配置, 可在任意位置获取配置
	config.AddConfigPath("./test/conf") //设置读取的文件路径
	config.SetConfigName("app")         //设置读取的文件名
	config.SetConfigType("yaml")
	if err := config.ReadInConfig(); err != nil {
		panic(err)
	}
}

var con *elastic.Client

func init() {
	// 初始化es连接
	options := []elastic.ClientOptionFunc{
		elastic.SetURL(viper.GetString("es.address")),
		elastic.SetSniff(viper.GetBool("es.sniff")),
		elastic.SetHealthcheckInterval(10 * time.Second),
		elastic.SetGzip(viper.GetBool("es.gzip")),
		elastic.SetErrorLog(log.New(os.Stderr, "ELASTIC ", log.LstdFlags)),
		elastic.SetInfoLog(log.New(os.Stdout, "", log.LstdFlags)),
		elastic.SetBasicAuth(viper.GetString("es.username"), viper.GetString("es.password")),
	}
	var err error
	con, err = elastic.NewClient(options...)
	if err != nil {
		panic(err)
	}
}

func Es() *elastic.Client {
	return con
}
