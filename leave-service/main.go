package main

import (
	"log"
	"net/http"
	"os"

	"github.com/asdlc-repos/mxcz308/leave-service/internal/handlers"
	"github.com/asdlc-repos/mxcz308/leave-service/internal/middleware"
	"github.com/asdlc-repos/mxcz308/leave-service/internal/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}
	addr := ":" + port

	s := store.New()
	h := handlers.New(s)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Apply middleware: CORS (outermost) → structured request/response logging
	srv := middleware.Chain(mux, middleware.CORS, middleware.Logging)

	log.Printf("INFO leave-service starting on %s", addr)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatalf("FATAL server error: %v", err)
	}
}
