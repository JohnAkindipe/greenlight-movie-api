package main

import (
	"errors"
	"expvar"
	"fmt"
	"greenlight-movie-api/internal/data"
	"greenlight-movie-api/internal/validator"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tomasen/realip"
	"golang.org/x/time/rate"
)

type IPAddr string

type metricsResponseWriter struct {
	wrapped       http.ResponseWriter
	statusCode    int
	headerWritten bool
}

/*********************************************************************************************************************/
/*
METRICS SPECIFIC FUNCTIONS
*/
func newMetricsResponseWriter(w http.ResponseWriter) *metricsResponseWriter {
	return &metricsResponseWriter{
		wrapped:       w,
		statusCode:    http.StatusOK,
		headerWritten: false,
	}
}

// The Header() method is a simple 'pass through' to the Header() method of the
// wrapped http.ResponseWriter.
func (mw *metricsResponseWriter) Header() http.Header {
	return mw.wrapped.Header()
}

// Likewise the Write() method does a 'pass through' to the Write() method of the
// wrapped http.ResponseWriter. Calling this will automatically write any
// response headers, so we set the headerWritten field to true.
func (mw *metricsResponseWriter) Write(b []byte) (int, error) {
	mw.headerWritten = true
	return mw.wrapped.Write(b)
}

// Again, the WriteHeader() method does a 'pass through' to the WriteHeader()
// method of the wrapped http.ResponseWriter. But after this returns,
// we also record the response status code (if it hasn't already been recorded)
// and set the headerWritten field to true to indicate that the HTTP response
// headers have now been written.
func (mw *metricsResponseWriter) WriteHeader(statusCode int) {
	mw.wrapped.WriteHeader(statusCode)
	if !mw.headerWritten {
		mw.statusCode = statusCode
		mw.headerWritten = true
	}
}

func (mw *metricsResponseWriter) Unwrap() http.ResponseWriter {
	return mw.wrapped
}

/*********************************************************************************************************************/
/*
RECOVER PANIC
When we do middleware chaining in Go, the stack is only unwound if there is a panic during the
course of responding to the request, if any middleware panics, the stack is unwound from that point.
This will unwind the stack for the affected goroutine (calling any deferred functions along the way), close the
underlying HTTP connection, and log an error message and stack trace.
*/
func (appPtr *application) recoverPanic(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		defer func() {
			if err := recover(); err != nil {
				/*
									If there was a panic, set a "Connection: close" header on the
					                response. This acts as a trigger to make Go's HTTP server
					                automatically close the current connection after a response has been
					                sent. Refer to notes(1) below
				*/
				w.Header().Set("Connection", "close")
				/*
				   The value returned by recover() has the type any, so we use
				   fmt.Errorf() to normalize it into an error and call our
				   serverErrorResponse() helper. In turn, this will log the error using
				   our custom Logger type at the ERROR level and send the client a 500
				   Internal Server Error response.
				*/
				appPtr.serverErrorResponse(w, r, fmt.Errorf("%s", err))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

/*********************************************************************************************************************/
//Read Notes(3) for more information on the limitation of using this pattern
//for rate-limiting
func (appPtr *application) rateLimit(next http.Handler) http.Handler {
	//code here will only run once, the first time this function is called.'
	//this code is universal in our server, all requests entering our server
	//will all share the logic dictated here
	type clientInfo struct {
		limiterPtr *rate.Limiter
		lastSeen   time.Time
	}

	//we need this mutex to synchronize access to the ipClientInfo map
	var mut sync.Mutex
	ipClientInfoMap := map[IPAddr]*clientInfo{}

	//Set a new global rate limiter that only allows 100 requests in one sec
	//It fills back the bucket with 25 allowances per second.
	globalLimiter := rate.NewLimiter(25, 100)

	//if we don't want to rateLimit - shouldRateLimit is a boolean
	if appPtr.config.rateLimit.shouldRateLimit {
		//background goroutine to run every minute and delete stale ipAddreses from
		//the ipClientInfo map, this is necessary to prevent the in-memory
		//app growing to large and consuming too much memory. Think of this
		//like a make-shift garbage collector
		go func() {
			for {
				time.Sleep(1 * time.Minute)
				// fmt.Println("cleaning up clientInfoMap")
				mut.Lock()
				//delete the ip and corresponding clientInfo from the
				//clientInfo map, if the ip has not been seen in the
				//last 3 minutes.
				for IPAddr, clientInfo := range ipClientInfoMap {
					if time.Since(clientInfo.lastSeen) > 3*time.Minute {
						delete(ipClientInfoMap, IPAddr)
					}
				}
				mut.Unlock()
			}
		}()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//code in here will be run in a different goroutine for every request
		//i.e. it will be request-specific.
		if appPtr.config.rateLimit.shouldRateLimit {
			//check if the request should continue as dictated by our global rate limiter
			if !globalLimiter.Allow() {
				appPtr.globalRateLimitExceededResponse(w, r)
				return
			}
			//retrieve the ip address from the request
			// ip, _, err := net.SplitHostPort(r.RemoteAddr)
			// if err != nil {
			// 	appPtr.serverErrorResponse(w, r, err)
			// 	return
			// }

			ip := realip.FromRequest(r)

			//cast ipAddr from type string to type IPAddr
			ipAddr := IPAddr(ip)

			//maps are not safe for concurrent use, hence we need to use
			//a mutex lock when we want to work with this map to prevent
			//concurrent access.
			//If the ipAddr doesn't already exist as a key in our map, add it to the map
			//and create a clientInfo struct with its own limiter and set the lastSeen
			//field to current time.
			mut.Lock()
			if _, exists := ipClientInfoMap[ipAddr]; !exists {
				ipClientInfoMap[ipAddr] = &clientInfo{
					limiterPtr: rate.NewLimiter(
						rate.Limit(appPtr.config.rateLimit.individualReqFillRate),
						appPtr.config.rateLimit.maxIndividualBurstReq,
					),
				}
			}

			//we have to update the last seen here to cater for the condition where
			//the ipAddress does exist in the ipClientInfo map
			ipClientInfoMap[ipAddr].lastSeen = time.Now()

			//client info contains the limiter for the client
			//and the last seen
			clientInfo := ipClientInfoMap[ipAddr]

			//check if the limiter for that client allows execution to continue
			//send a too many requests response to the specific client otherwise.
			if !clientInfo.limiterPtr.Allow() {
				mut.Unlock()
				appPtr.rateLimitExceededResponse(w, r)
				return
			}

			mut.Unlock()
		}
		next.ServeHTTP(w, r)
	})
}

/*********************************************************************************************************************/
func (appPtr *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse the JWT and extract the claims. This will return an error if the JWT
		// contents doesn't match the signature (i.e. the token has been tampered with)
		// or the algorithm isn't valid.
		/******************************************************************************/
		// Add the "Vary: Authorization" header to the response. This indicates to any
		// caches that the response may vary based on the value of the Authorization
		// header in the request.
		w.Header().Add("Vary", "Authorization")
		// Retrieve the value of the Authorization header from the request. This will
		// return the empty string "" if there is no such header found.
		authorizationHeader := r.Header.Get("Authorization")
		// If there is no Authorization header found, use the contextSetUser() helper
		// that we just made to add the AnonymousUser to the request context. Then we
		// call the next handler in the chain and return without executing any of the
		// code below.
		if authorizationHeader == "" {
			r = appPtr.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}
		// Otherwise, we expect the value of the Authorization header to be in the format
		// "Bearer <token>". We try to split this into its constituent parts, and if the
		// header isn't in the expected format we return a 401 Unauthorized response
		// using the invalidAuthenticationTokenResponse() helper
		headerParts := strings.Split(authorizationHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			appPtr.invalidAuthenticationTokenResponse(w, r)
			return
		}
		// Extract the actual authentication token from the header parts.
		token := headerParts[1]

		// Validate the token to make sure it is in a sensible format.
		tokenValidator := validator.New()

		// If the token isn't valid, use the invalidAuthenticationTokenResponse()
		// helper to send a response, rather than the failedValidationResponse() helper
		// that we'd normally use.
		data.ValidateToken(tokenValidator, token)
		if !tokenValidator.Valid() {
			appPtr.invalidAuthenticationTokenResponse(w, r)
			return
		}
		// Retrieve the details of the user associated with the authentication token,
		// again calling the invalidAuthenticationTokenResponse() helper if no
		// matching record was found. IMPORTANT: Notice that we are using
		// ScopeAuthentication as the first parameter here.
		userPtr, err := appPtr.dbModel.UserModel.GetForToken(data.ScopeAuthentication, token)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrRecordNotFound):
				appPtr.invalidAuthenticationTokenResponse(w, r)
			default:
				appPtr.serverErrorResponse(w, r, err)
			}
			return
		}
		r = appPtr.contextSetUser(r, userPtr)
		next.ServeHTTP(w, r)

		/******************************************************************************/
		/*
			ALL OF THE BELOW IS FOR THE JWT OPTION OF TOKEN AUTHENTICATION
		*/
		// // Parse the JWT and extract the claims. This will return an error if the JWT
		// // contents doesn't match the signature (i.e. the token has been tampered with)
		// // or the algorithm isn't valid.
		// claims, err := jwt.HMACCheck([]byte(token), []byte(appPtr.config.jwt.secret))
		// if err != nil {
		// 	//TODO: appPtr.invalidAuthenticationTokenResponse(w, r)
		// 	return
		// }
		// //Check if the JWT is still valid at this moment in time.
		// if !claims.Valid(time.Now()) {
		// 	//TODO: appPtr.invalidAuthenticationTokenResponse(w, r)
		// 	return
		// }
		// //Check that the issuer is our application.
		// if claims.Issuer != "greenlight.akindipe.john" {
		// 	//TODO: appPtr.invalidAuthenticationTokenResponse(w, r)
		// 	return
		// }
		// if !claims.AcceptAudience("greenlight.akindipe.john") {
		// 	//TODO: appPtr.invalidAuthenticationTokenResponse(w, r)
		// 	return
		// }
		// // At this point, we know that the JWT is all OK and we can trust the data in
		// // it. We extract the user ID from the claims subject and convert it from a
		// // string into an int64. TODO: Uncomment the below line
		// //userID, err := strconv.ParseInt(claims.Subject, 10, 64)
		// if err != nil {
		// 	appPtr.serverErrorResponse(w, r, err)
		// 	return
		// }

		// // Lookup the user record from the database
		// //TODO: Uncomment the below line when I use "user"
		// //user, err := appPtr.dbModel.UserModel.Get(userID)
		// if err != nil {
		// 	switch {
		// 	case errors.Is(err, data.ErrRecordNotFound):
		// 		// TODO: app.invalidAuthenticationTokenResponse(w, r)
		// 	default:
		// 		appPtr.serverErrorResponse(w, r, err)
		// 	}
		// 	return
		// }

		// // Add the user record to the request context and continue as normal.
		// //TODO: Implement app.contextSetUser
		// // r = app.contextSetUser(r, user)
		// next.ServeHTTP(w, r)
	})
}

/*********************************************************************************************************************/
func (appPtr *application) requireAuthenticatedUser(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			userPtr := appPtr.contextGetUser(r)
			if userPtr.IsAnonymous() {
				appPtr.authenticationRequiredResponse(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
}

/*********************************************************************************************************************/
func (appPtr *application) requireActivatedUser(next http.HandlerFunc) http.HandlerFunc {
	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userPtr := appPtr.contextGetUser(r)
		if !userPtr.Activated {
			appPtr.activationRequiredResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
	return appPtr.requireAuthenticatedUser(fn)
}

/*********************************************************************************************************************/
/*
The REQUIRE PERMISSION middleware will take in a specified permission and check if the user currently making
a request has the specified permission to complete the request.
It will automatically wrap the requireActivatedUser() middleware which already wraps the
requireAuthenticatedUser() middleware.
*/
func (appPtr *application) requirePermission(permission string, next http.HandlerFunc) http.HandlerFunc {
	fn := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			userPtr := appPtr.contextGetUser(r) //we're sure we have a genuine user at this point
			permissions, err := appPtr.dbModel.PermissionModel.GetAllForUser(userPtr.ID)
			if err != nil {
				appPtr.serverErrorResponse(w, r, err)
				return
			}
			if !permissions.Include(permission) {
				appPtr.notPermittedResponse(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})

	return appPtr.requireActivatedUser(fn)
}

/*********************************************************************************************************************/
/*
The ENABLE CORS middleware will tell browswers which origins are allowed to read responses from our server.
Currently, all origins are allowed to read responses from our server.
This header is only enforced on browsers and will not affect requests from other sources outside a browser
such as the command line.
*/
func (appPtr *application) enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin") //See notes(4)
		w.Header().Add("Vary", "Access-Control-Request-Method")
		origin := r.Header.Get("Origin")

		if slices.Contains(appPtr.config.cors.trustedOrigins, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		//allow request proceed as nrmal if no match found, thus
		//the request will default to only same-site origin allowed
		//for CORS

		//Identify request as pre-flight See notes(5)
		if r.Method == http.MethodOptions {
			if origin != "" {
				if r.Header.Get("Access-Control-Request-Method") != "" {
					// This is a pre-flight request
					w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, PUT, PATCH, DELETE")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

					w.WriteHeader(http.StatusOK)
					return
				}
			}
		}
		//Not pre-flight request
		next.ServeHTTP(w, r)
	})
}

/*********************************************************************************************************************/
/*
The METRICS middleware will generate per-request metrics for our application.
*/
func (appPtr *application) metrics(next http.Handler) http.Handler {
	// The below variables will be created only once: when the middleware chain is built
	// Refer Notes(6)
	var (
		totalRequestsReceived = expvar.NewInt("total-requests-received")
		//by the time we send a response, this response won't be included in this variable
		//it will be incremented in a later fashion i.e. If we receive 2 responses, it will
		//show that we have received only 1. Likewise the totalProcessingTime. This is
		//happening because we are incrementing them after the response has been sent.
		totalResponsesSent              = expvar.NewInt("total_responses_sent")
		totalProcessingTimeMicroseconds = expvar.NewInt("total_processing_time_μs")
		// Declare a new expvar map to hold the count of responses for each HTTP status
		// code.
		totalResponsesSentByStatus = expvar.NewMap("total_responses_sent_by_status")
	)

	//This will run on per-request basis. The variables above are safe for concurrent use
	//and can be incremented and decremented by different goroutines without data races.
	//This means, they must have a mutex-like implementation internally
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		totalRequestsReceived.Add(1)

		mwPtr := newMetricsResponseWriter(w)
		//I use a defer here because I want the processingTime to increase
		//whether or not the request completed successfully or we returned
		//an error such as a panic.
		defer func() {
			processingTime := time.Since(start).Microseconds()
			totalProcessingTimeMicroseconds.Add(processingTime)

			totalResponsesSentByStatus.Add(strconv.Itoa(mwPtr.statusCode), 1)
			totalResponsesSent.Add(1)
		}()

		next.ServeHTTP(mwPtr, r)
	})
}

/*
1 CLOSE CONNECTION MANUALLY
Panic would usually unwind the entire goroutine stack, call
deferred functions along the way, close the underlying http connection (without sending a message),
print the stack trace and log the error message. However, the documentatino states that if we recover
a panic in a deferred function (as we are doing in this case), the panicking sequence stops, therefore
the onus is on us to manually take over what the panicking sequence will normally do. Hence, we
- Close the http connection
- log the error and
- Send an error response (the panicking sequence wouldn't normally do this, however, we can
since we have taken over the panicking sequence) using our custom serverErrorResponse helper

2 PANIC RECOVERY IN OTHER GOROUTINES
It’s really important to realize that our middleware will only recover panics that happen
in the same goroutine that executed the recoverPanic() middleware.

If, for example, you have a handler which spins up another goroutine (e.g. to do some background
processing), then any panics that happen in the background goroutine will not be recovered — not
by the recoverPanic() middleware… and not by the panic recovery built into http.Server. These
panics will cause your application to exit and bring down the server.

So, if you are spinning up additional goroutines from within your handlers and there is any chance
of a panic, you must make sure that you recover any panics from within those goroutines too.

3. RATE LIMITING USING THE PATTERN DESIGNED ABOVE
Using this pattern for rate-limiting will only work if your API application is running on a
single-machine. If your infrastructure is distributed, with your application running on multiple
servers behind a load balancer, then you’ll need to use an alternative approach.

If you’re using HAProxy or Nginx as a load balancer or reverse proxy, both of these have built-in
functionality for rate limiting that it would probably be sensible to use. Alternatively, you
could use a fast database like Redis to maintain a request count for clients, running on a server
which all your application servers can communicate with.

4 VARY RESPONSE HEADER
If your code makes a decision about what to return based on the content of a request header, you
should include that header name in your Vary response header — even if the request didn’t include
that header. This is important for CACHES and prevents subtle bugs like as described below
https://textslashplain.com/2018/08/02/cors-and-vary/s

5. IDENTIFYING AND RESPONDING TO PREFLIGHT REQUESTS
A browser will send a preflight request when it determines that the real request it wants to make
is not a simple CORS request.
Preflight requests always have three components: the HTTP method OPTIONS, an Origin header, and an
Access-Control-Request-Method header. If any one of these pieces is missing, we know that it is not
a preflight request.

Once we identify that it is a preflight request, we need to send a 200 OK response with some special
headers to let the browser know whether or not it’s OK for the real request to proceed. These are:
- An Access-Control-Allow-Origin response header, which reflects the value of the preflight request’s
Origin header (just like in the previous chapter).
- An Access-Control-Allow-Methods header listing the HTTP methods that can be used in real cross-origin
requests to the URL.
- An Access-Control-Allow-Headers header listing the request headers that can be included in real
cross-origin requests to the URL.

When responding to a preflight request it’s not necessary to include the CORS-safe methods HEAD, GET or
POST in the Access-Control-Allow-Methods header. Likewise, it’s not necessary to include forbidden or
CORS-safe headers in Access-Control-Allow-Headers.

6. MIDDLEWARE UP THE CALL STACK
When we call the next.ServeHTTP() inside the middleware, it will call the next middleware, when that
middleware returns, we increase the responses sent variable (at this point we know we have successfully
sent a response, hence why we can't use a defer - a defer would increment the response sent, whether or
not we send a response. The author doesn't do this, but I believe we should increase the
totalProcessingTime in a defer because, we want to increase it whether or not we were able to send a
response successfully or not). Another clever thing we do is to initialize the variables before we return
the main body of the middleware; this is important because go serves every request in a new goroutine.
If we initialize the variable inside the main middleware body, it will initialize a different variable for
every request, but by doing it as we have currently done (outside the returned middleware), the variables
will be global for the application and every request (in another goroutine) will be referencing the same
global variables and making changes to the variables.

OTHER USEFUL METRICS:
- The number of ‘active’ in-flight requests:
total_requests_received - total_responses_sent
- The average number of requests received per second (between calls A and B to the GET /debug/vars endpoint):
(total_requests_received_B - total_requests_received_A) / (timestamp_B - timestamp_A)
- The average processing time per request (between calls A and B to the GET /debug/vars endpoint):
(total_processing_time_μs_B - total_processing_time_μs_A) / (total_requests_received_B - total_requests_received_A)
*/
