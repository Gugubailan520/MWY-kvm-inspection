// kvm-server 入口
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kvm-inspection/Server/internal/api"
	"github.com/kvm-inspection/Server/internal/auth"
	"github.com/kvm-inspection/Server/internal/config"
	"github.com/kvm-inspection/Server/internal/ifstatstore"
	"github.com/kvm-inspection/Server/internal/logstore"
	"github.com/kvm-inspection/Server/internal/model"
	"github.com/kvm-inspection/Server/internal/service"
	"github.com/kvm-inspection/Server/internal/violation"
	"github.com/kvm-inspection/Server/internal/ws"
)

const version = "0.1.0"

func main() {
	cfgPath := flag.String("c", "config/server.local.yaml", "config file path")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 1) MySQL
	db, err := gorm.Open(mysql.Open(cfg.MySQL.String()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("connect mysql: %v", err)
	}
	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	svc := service.New(db)
	if err := svc.EnsureAdmin("admin123"); err != nil {
		log.Printf("ensure admin: %v", err)
	}

	// 2) MongoDB
	ls, err := logstore.New(ctx, cfg.Mongo.URI, cfg.Mongo.Database, cfg.Mongo.Collection)
	if err != nil {
		log.Fatalf("connect mongo: %v", err)
	}
	ifs, err := ifstatstore.New(ctx, ls.Database(), cfg.Mongo.IfStatColl)
	if err != nil {
		log.Fatalf("init ifstat store: %v", err)
	}

	// 3) 规则引擎（预热）
	eng := violation.New()
	if rs, err := svc.ListRules(); err == nil {
		eng.Reload(rs)
	}

	// 4) Hub
	hub := ws.NewHub()

	// 5) Auth
	am := auth.New(cfg.JWT.Secret, cfg.JWT.ExpireHour)

	// 6) HTTP
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	h := api.New(svc, ls, ifs, eng, hub, am)
	h.Register(r, cfg.Server.CORSOrigins)

	srv := &http.Server{
		Addr:    cfg.Server.HTTPAddr,
		Handler: r,
	}

	go func() {
		log.Printf("[server] listening on %s, version=%s", cfg.Server.HTTPAddr, version)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("[server] shutting down...")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	_ = srv.Shutdown(shutCtx)
}
