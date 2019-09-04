package restful

import (
	"context"
	"log"
	"net/http"

	"oakv/engine"
)

func ListenAndServe(ctx context.Context, server *engine.Server) error {
	webSvr := http.Server{
		Addr: ":2345",
	}
	go func() {
		log.Println("restful listening")
		if err := webSvr.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalln("restful engine error", err)
		}
	}()

	select {
	case <-ctx.Done():
		log.Println("closing restful")
		return webSvr.Shutdown(ctx)
	}
}
