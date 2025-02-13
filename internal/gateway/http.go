package gateway

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	maxInsertRequestSize = 128 * 1024 * 1024 // 128 MB
)

func (g *Gateway) ListenAndServe() error {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "prometheus-mimic-gateway")
	})

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	router.POST("/api/v1/write", g.basicAuthMiddleware(), writeHeadersMiddleware, g.writeHandler)

	listenAddr, ok := os.LookupEnv("LISTEN_ADDRESS")
	if !ok {
		listenAddr = ":8080"
	}

	srv := &http.Server{
		Addr:    listenAddr,
		Handler: router.Handler(),
	}

	go func() {
		defer g.kafkaProducer.Close()
		defer g.kafkaClient.Close()

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutdown server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown: %s", err.Error())
	}

	return nil
}
