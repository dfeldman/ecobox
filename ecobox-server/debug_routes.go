package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

func main() {
	router := mux.NewRouter()
	
	// Test exact same setup as the application
	api := router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "API route working!")
	}).Methods("GET")
	
	api.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Metrics route working!")
	}).Methods("GET")
	
	fmt.Println("Starting debug server on :8082")
	fmt.Println("Test routes:")
	fmt.Println("  http://localhost:8082/api/test")
	fmt.Println("  http://localhost:8082/api/metrics")
	
	http.ListenAndServe(":8082", router)
}
