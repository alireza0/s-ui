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

	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/nodeagent"
	"github.com/op/go-logging"
)

func main() {
	state := flag.String("state", "", "private persistent state directory")
	nodeID := flag.Uint64("node-id", 0, "node identity")
	listen := flag.String("listen", "127.0.0.1:16443", "mTLS control listener")
	cert := flag.String("tls-cert", "", "control server certificate")
	key := flag.String("tls-key", "", "control server private key")
	ca := flag.String("client-ca", "", "CA allowed to control this node")
	bootstrap := flag.String("bootstrap-id", "", "Installation operation requiring fresh access once")
	flag.Parse()
	if *state == "" || *nodeID == 0 || *cert == "" || *key == "" || *ca == "" {
		log.Fatal("state, node-id, tls-cert, tls-key and client-ca are required")
	}
	logger.InitLogger(logging.ERROR)
	nodeagent.ExecutableDigest()
	canonicalState, err := nodeagent.CanonicalStateDirectory(*state)
	if err != nil {
		log.Fatal(err)
	}
	*state = canonicalState
	tlsConfig, err := nodeagent.ServerTLS(*cert, *key, *ca)
	if err != nil {
		log.Fatal(err)
	}
	runtime := nodeagent.NewCoreRuntimeWithState(*state)
	manager := nodeagent.NewManagerV2(*nodeID, nodeagent.FileStoreV2{Directory: *state}, runtime)
	manager.RequireBootstrap(*bootstrap)
	if err := manager.Restore(); err != nil {
		log.Fatal(err)
	}
	defer manager.Close()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	server := &http.Server{Addr: *listen, Handler: nodeagent.HandlerV2(manager), TLSConfig: tlsConfig,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runtime.SampleTraffic()
				if err := manager.Refresh(); err != nil {
					log.Print("could not apply known expiry")
				}
			}
		}
	}()
	go func() {
		<-ctx.Done()
		shutdown, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("node %d control listening on %s", *nodeID, *listen)
	if err := server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
