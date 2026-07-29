// Package config 定义 Server 配置。
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config Server 配置
type Config struct {
	Server ServerCfg `yaml:"server"`
	MySQL  DSN       `yaml:"mysql"`
	Mongo  MongoCfg  `yaml:"mongo"`
	JWT    JWTCfg    `yaml:"jwt"`
	Log    LogCfg    `yaml:"log"`
}

type ServerCfg struct {
	HTTPAddr    string   `yaml:"http_addr"` // :8080
	CORSOrigins []string `yaml:"cors_origins"`
}

type DSN struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

func (d DSN) String() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		d.User, d.Password, d.Host, d.Port, d.DBName)
}

type MongoCfg struct {
	URI        string `yaml:"uri"`
	Database   string `yaml:"database"`
	Collection string `yaml:"collection"`
	IfStatColl string `yaml:"ifstat_collection"` // 接口流量快照集合，默认 interface_stats
}

type JWTCfg struct {
	Secret     string `yaml:"secret"`
	ExpireHour int    `yaml:"expire_hour"`
}

type LogCfg struct {
	Level string `yaml:"level"` // debug | info | warn | error
}

// Load 加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	c.applyDefaults()
	return &c, nil
}

func (c *Config) applyDefaults() {
	if c.Server.HTTPAddr == "" {
		c.Server.HTTPAddr = ":8080"
	}
	if c.Mongo.Database == "" {
		c.Mongo.Database = "kvm_inspection"
	}
	if c.Mongo.Collection == "" {
		c.Mongo.Collection = "network_events"
	}
	if c.Mongo.IfStatColl == "" {
		c.Mongo.IfStatColl = "interface_stats"
	}
	if c.JWT.ExpireHour <= 0 {
		c.JWT.ExpireHour = 24
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
}
