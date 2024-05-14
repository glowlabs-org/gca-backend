package server

import (
	"net"
	"net/http"
)

// launchAPI sets up the HTTP API endpoints and starts the HTTP server.
// This function initializes the API routes and starts the HTTP server.
func (gcas *GCAServer) launchAPI() {
	// Attach all of the handlers to the mux.
	gcas.mux.HandleFunc("/api/v1/all-device-stats", gcas.AllDeviceStatsHandler)
	gcas.mux.HandleFunc("/api/v1/authorized-servers", gcas.AuthorizedServersHandler)
	gcas.mux.HandleFunc("/api/v1/authorize-equipment", gcas.AuthorizeEquipmentHandler)
	gcas.mux.HandleFunc("/api/v1/equipment", gcas.EquipmentHandler)
	gcas.mux.HandleFunc("/api/v1/equipment-migrate", gcas.EquipmentMigrateHandler)
	gcas.mux.HandleFunc("/api/v1/register-gca", gcas.RegisterGCAHandler)
	gcas.mux.HandleFunc("/api/v1/recent-reports", gcas.RecentReportsHandler)
	gcas.mux.HandleFunc("/api/v1/geo-stats", gcas.GeoStatsHandler)
	// Internal APIs which will not be accessible except under bench testing mode
	gcas.mux.HandleFunc("/api/int/wt-historical", gcas.InternalWattTimeHistoricalHandler)
	gcas.mux.HandleFunc("/api/int/wt-signal-index", gcas.InternalWattTimeSignalIndexHandler)

	// Create a listener. In prod it's a specfic port, during testing it's
	// ":0". Because we don't know what the port is during testing, we need
	// to build the listener manually so that we can grab the port from it.
	listener, err := net.Listen("tcp", gcas.httpServer.Addr)
	if err != nil {
		panic("unable to launch gca api")
	}
	gcas.httpPort = uint16(listener.Addr().(*net.TCPAddr).Port)

	// Launch the background thread that keeps the API running. The
	// listener gets handed off to the httpServer, which will be
	// responsible for closing the listener, therefore the listener does
	// not need to be closed here. If the Launch fails, the listener will
	// never get attached to the httpServer, which means we will have to
	// close it manually.
	err = gcas.tg.Launch(func() {
		gcas.logger.Info("Starting HTTP server on ", gcas.httpServer.Addr)
		if err := gcas.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			gcas.logger.Fatal("Could not start HTTP server: ", err)
		}
	})
	if err != nil {
		listener.Close()
	}
}
