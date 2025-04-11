package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

/*********************************************************************************************************************/
// VERSION CONSTANT
// hard coded version constant, we'll automatically determine this later
const version = "1.0.0"
/*********************************************************************************************************************/
// Define a config struct to hold all the configuration settings for our application.
// For now, the only configuration settings will be the network port that we want the
// server to listen on, and the name of the current operating environment for the
// application (development, staging, production, etc.). We will read in these
// configuration settings from command-line flags when the application starts.
type config struct {
    port int
    env  string
}
/*********************************************************************************************************************/
// Define an application struct to hold the dependencies for our HTTP handlers, helpers,
// and middleware. At the moment this only contains a copy of the config struct and a 
// logger, but it will grow to include a lot more as our build progresses.
type application struct {
    config config
    logger *slog.Logger
}
/*********************************************************************************************************************/
// MAIN FUNC
func main() {
/*********************************************************************************************************************/	
	// Declare an instance of the config struct.
	var cfg config
/*********************************************************************************************************************/
	// COMMAND LINE FLAGS
	// Use flags to get the value for variables we'll use in our application from command-line flags.
	// The IntVar and StringVar will automatically store the result of the flag in the destination
	flag.IntVar(&cfg.port, "port", 3000, "This value specifies what port the server should listen on")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")
	flag.Parse()
/*********************************************************************************************************************/
	// LOG SETUP
	// Initialize a new structured logger which writes log entries to the standard out 
    // stream.
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
/*********************************************************************************************************************/
	// APP STRUCT SETUP
	// Initialize the application with the config and logger we've set up
	/*
	We'll also define all our route handlers on this application struct using a pointer receiver,
	this way all dependences needed by our handlers can be provided as a field in the application
	without resorting to global variables or closures
	*/
	appPtr := &application{
		config: cfg,
		logger: logger,
	}
/*********************************************************************************************************************/
	// MUX SETUP
	// Declare a new servemux and add a /v1/healthcheck route which dispatches requests
    // to the healthcheckHandler method (which we will create in a moment).	
	muxPtr := http.NewServeMux()
	muxPtr.HandleFunc("/v1/healthcheck", appPtr.healthcheckHandler)
/*********************************************************************************************************************/
	// SERVER SETUP
	srvPtr := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.port),
		Handler: muxPtr,
		IdleTimeout: time.Minute,
		ReadTimeout: 5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
/*********************************************************************************************************************/
	// SERVER START THE HTTP SERVER
	//log that we're starting the server at this port and in this environment
	logger.Info("starting server", "addr", srvPtr.Addr, "env", cfg.env)
	// call the listen and serve method of srvPtr
	err := srvPtr.ListenAndServe()
	// log error explaining why the serve failed to run, if any
	logger.Error(err.Error())
/*********************************************************************************************************************/
	// STOP THE SERVER
	os.Exit(1)
}

