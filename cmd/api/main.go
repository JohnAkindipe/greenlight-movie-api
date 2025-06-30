package main

import (
	"context"
	"database/sql"
	"flag"
	"greenlight-movie-api/internal/data"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

/*********************************************************************************************************************/
// VERSION CONSTANT
// hard coded version constant, we'll automatically determine this later
const version = "1.0.0"
/*********************************************************************************************************************/
// SETUP CONFIGURATION
// Define a config struct to hold all the configuration settings for our application.
// For now, the only configuration settings will be the network port that we want the
// server to listen on, and the name of the current operating environment for the
// application (development, staging, production, etc.). We will read in these
// configuration settings from command-line flags when the application starts.
// Add a db struct field to hold the configuration settings for our database connection
// pool. For now this only holds the DSN, which we will read in from a command-line flag.
type config struct {
    port int
    env  string
	db struct {
		dsn string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime time.Duration
	}
	rateLimit struct {
		maxGlobalBurstReq	int
		globalReqFillRate	float64
		maxIndividualBurstReq int
		individualReqFillRate float64
		shouldRateLimit bool
	}
}
/*********************************************************************************************************************/
// APPLICATION CONFIGURATION
// Define an application struct to hold the dependencies for our HTTP handlers, helpers,
// and middleware. At the moment this contains a copy of the config struct, a copy of
// the data.Models struct and a logger, but it will grow to include a lot more as our 
// build progresses.
type application struct {
    config config
    logger *slog.Logger
	dbModel data.Models
}
/*********************************************************************************************************************/
// OPEN DB to open a connection pool
// The openDB() function returns a sql.DB connection pool.
func openDB(cfg config) (*sql.DB, error) {
    // Use sql.Open() to create an empty connection pool, using the DSN from the config
    // struct.
    dbPtr, err := sql.Open("postgres", cfg.db.dsn)
    if err != nil {
        return nil, err
    }

	// Set the maximum idle timeout for connections in the pool. Passing a duration less
    // than or equal to 0 will mean that connections are not closed due to their idle time. 
	dbPtr.SetConnMaxIdleTime(cfg.db.maxIdleTime)
    // Set the maximum number of idle connections in the pool. Again, passing a value
    // less than or equal to 0 will mean there is no limit.
	dbPtr.SetMaxIdleConns(cfg.db.maxIdleConns)
	// Set the maximum number of open (in-use + idle) connections in the pool. Note that
    // passing a value less than or equal to 0 will mean there is no limit.
	dbPtr.SetMaxOpenConns(cfg.db.maxOpenConns)
    

    // Create a context with a 5-second timeout deadline.
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Use PingContext() to establish a new connection to the database, passing in the
    // context we created above as a parameter. If the connection couldn't be
    // established successfully within the 5 second deadline, then this will return an
    // error. If we get this error, or any other, we close the connection pool and 
    // return the error.
    err = dbPtr.PingContext(ctx)
    if err != nil {
        dbPtr.Close()
        return nil, err
    }

    // Return the *sql.DB connection pool.
    return dbPtr, nil
}
/*********************************************************************************************************************/
/*
GETINTENVVAR
This is a function to get environment variables which are 
stored as strings and convert them to integers
for environment variables that need to be used as integers
*/
func getIntEnvVars(intEnvs *map[string]int, loggerPtr *slog.Logger) {
	for varName := range *intEnvs {
		envVar, err := strconv.Atoi(os.Getenv(varName))
		if err != nil {
			loggerPtr.Error(err.Error())
			os.Exit(1)
		}
		(*intEnvs)[varName] = envVar
	}
}
/*********************************************************************************************************************/
// MAIN FUNC
func main() {
	// LOG SETUP
	// Initialize a new structured logger which writes log entries to the standard out 
    // stream.
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
/*********************************************************************************************************************/
	//LOAD ENVIRONMENT VARIABLES
	// Log error and exit if there was an error loading the environment variables
	err := godotenv.Load()
	
	if err != nil {
		logger.Error("Failed to load env variables", "err", err.Error())
		os.Exit(1)
	}
	// Get the maxIdleConns, maxOpenConns, maxGlobalBurstReq, globalReqFillRate
	// maxIndividualBurstReq, individualReqFillRate from the env variables
	// and convert them to integers

	intEnvs := map[string]int{
		"MAXIDLECONNS":0,
		"MAXOPENCONNS":0,
		"MAXGLOBALBURSTREQ":0,
		"FILLRATEGLOBALREQ":0,
		"MAXINDIVIDUALBURSTREQ":0,
		"FILLRATEINDIVIDUALREQ":0,
		"DEFAULTPORT":0,
	}
	getIntEnvVars(&intEnvs, logger)
/*********************************************************************************************************************/	
	// Declare an instance of the config struct.
	var cfg config
/*********************************************************************************************************************/
	// COMMAND LINE FLAGS
	// Use flags to get the value for variables we'll use in our application from command-line flags.
	// The IntVar and StringVar will automatically store the result of the flag in the destination
	flag.IntVar(&cfg.port, "port", intEnvs["DEFAULTPORT"], "This value specifies what port the server should listen on")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")
	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("GREENLIGHT_DB_DSN"), "the dsn for the database")
	flag.DurationVar(&cfg.db.maxIdleTime, "conn-max-idle-time", 15 * time.Minute, "db conn-idle-timeout")
	flag.IntVar(&cfg.db.maxIdleConns, "max-idle-conns", intEnvs["MAXIDLECONNS"], "maximum no. of idle connections")
	flag.IntVar(&cfg.db.maxOpenConns, "max-open-conns", intEnvs["MAXOPENCONNS"], "maximum no. of db connections")
	flag.IntVar(&cfg.rateLimit.maxGlobalBurstReq, "max-global-burst-req", intEnvs["MAXGLOBALBURSTREQ"], "maximum no. of burst globhal reqs")
	flag.Float64Var(&cfg.rateLimit.globalReqFillRate, "global-req-fill-rate", float64(intEnvs["FILLRATEGLOBALREQ"]), "fill rate of global reqs")
	flag.IntVar(&cfg.rateLimit.maxIndividualBurstReq, "max-individual-burst-req", intEnvs["MAXINDIVIDUALBURSTREQ"], "maximum no. of burst individual reqs")
	flag.Float64Var(&cfg.rateLimit.individualReqFillRate, "individual-req-fill-rate", float64(intEnvs["FILLRATEINDIVIDUALREQ"]), "fill rate of individual reqs")
	flag.BoolVar(&cfg.rateLimit.shouldRateLimit, "should-rate-limit", true, "whether to allow rate-limiting")
	flag.Parse()
/*********************************************************************************************************************/
	// DATABASE SETUP
    // Call the openDB() helper function (see below) to create the connection pool,
    // passing in the config struct. If this returns an error, we log it and exit the
    // application immediately.
	dbPtr, err := openDB(cfg)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

    // Defer a call to db.Close() so that the connection pool is closed before the
    // main() function exits.
    defer dbPtr.Close()

    // Also log a message to say that the connection pool has been successfully 
    // established.
    logger.Info("database connection pool established")
/*********************************************************************************************************************/
	// APP STRUCT SETUP
	// Initialize the application with the config and logger we've set up
	/*
	We'll also define all our route handlers on this application struct using a pointer receiver,
	this way all dependences needed by our handlers can be provided as a field in the application
	without resorting to global variables or closures.
	Per the dbModel field on the appPtr struct, the function call will return a db model, initialized with a 
	moviesModel, whose dbPtr field is populated by the dbPtr we pass in
	*/
	appPtr := &application{
		config: cfg,
		logger: logger,
		dbModel: data.NewModel(dbPtr),
	}
/*********************************************************************************************************************/
	err = appPtr.serve()
/*********************************************************************************************************************/
	//I'm confused as to why we're checking if err is nil or not here
	//surely (I postulate), the listenandServe method is in an infinite 
	// loop, processing equests and never returns unless an error occurs
	// in which case, if the listenAndServe thus return, it certainly is
	// returning an error. This will only make sense, if it is possible
	// for listenandServe to return a nil error (perhaps with graceful 
	// shutdown?)
	if err != nil {
		// log error explaining why the server failed to run, if any
		logger.Error(err.Error())
	}
/*********************************************************************************************************************/
	// STOP THE SERVER
	os.Exit(1)
}

