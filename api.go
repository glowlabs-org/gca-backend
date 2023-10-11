package main

import (
	"net/http"
)

// launchAPI sets up the HTTP API endpoints and starts the HTTP server.
// This function initializes the API routes and starts the HTTP server.
func (gca *GCAServer) launchAPI() {
	gca.mux.HandleFunc("/api/v1/authorize-equipment", gca.AuthorizeEquipmentHandler)
	go func() {
		gca.logger.Info("Starting HTTP server on port 35015...")
		if err := gca.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			gca.logger.Fatal("Could not start HTTP server: ", err)
		}
	}()
}
