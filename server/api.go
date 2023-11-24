package server

import (
	"net"
	"net/http"
)

// launchAPI sets up the HTTP API endpoints and starts the HTTP server.
// This function initializes the API routes and starts the HTTP server.
func (gcas *GCAServer) launchAPI() {
	// Attach all of the handlers to the mux.
	gcas.mux.HandleFunc("/api/v1/authorized-servers", gcas.AuthorizedServersHandler)
	gcas.mux.HandleFunc("/api/v1/authorize-equipment", gcas.AuthorizeEquipmentHandler)
	gcas.mux.HandleFunc("/api/v1/equipment-migrate", gcas.EquipmentMigrateHandler)
	gcas.mux.HandleFunc("/api/v1/register-gca", gcas.RegisterGCAHandler)
	gcas.mux.HandleFunc("/api/v1/recent-reports", gcas.RecentReportsHandler)
	gcas.mux.HandleFunc("/api/v1/geo-stats", GeoStatsHandler)

	// Create a listener. In prod it's a specfic port, during testing it's
	// ":0". Because we don't know what the port is during testing, we need
	// to build the listener manually so that we can grab the port from it.
	listener, err := net.Listen("tcp", httpPort)
	if err != nil {
		panic("unable to launch gca api")
	}
	gcas.httpPort = uint16(listener.Addr().(*net.TCPAddr).Port)

	// Launch the background thread that keeps the API running.
	go func() {
		gcas.logger.Info("Starting HTTP server on port ", gcas.httpPort)
		if err := gcas.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			gcas.logger.Fatal("Could not start HTTP server: ", err)
		}
	}()
}
