package main

import (
    "log"
    "net/http"
    "os"
    
    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    
    "hotelmerp/internal/platform/auth"
    "hotelmerp/internal/distribution"
    "hotelmerp/internal/lodging"
    "hotelmerp/internal/finance"
    "hotelmerp/internal/mobile"
)

func main() {
    // Initialize core modules
    auth.Init()
    
    // Setup router
    r := chi.NewRouter()
    
    // Middleware
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    
    // API routes
    r.Route("/api/v1", func(r chi.Router) {
        // Distribution & Commerce - OTA integration
        distribution.Routes(r)
        
        // Lodging & Guest Operations
        lodging.Routes(r)
        
        // Finance & Accounting
        finance.Routes(r)
        
        // Mobile App API
        mobile.Routes(r)
    })
    
    // Start server
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }
    
    log.Printf("Server starting on port %s", port)
    log.Fatal(http.ListenAndServe(":"+port, r))
}
