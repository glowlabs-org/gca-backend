package main

import (
	"net"
	"net/http"
	"strconv"
)

// launchAPI sets up the HTTP API endpoints and starts the HTTP server.
// This function initializes the API routes and starts the HTTP server.
func (gcas *GCAServer) launchAPI() {
	gcas.mux.HandleFunc("/api/v1/authorize-equipment", gcas.AuthorizeEquipmentHandler)
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic("unable to launch gca api")
	}
	gcas.httpPort = ":" + strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	go func() {
		gcas.logger.Info("Starting HTTP server on port ", gcas.httpPort)
		if err := gcas.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			gcas.logger.Fatal("Could not start HTTP server: ", err)
		}
	}()
}
