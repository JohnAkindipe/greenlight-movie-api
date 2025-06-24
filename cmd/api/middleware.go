package main

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type IPAddr string

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

//Read Notes(3) for more information on the limitation of using this pattern
//for rate-limiting
func (appPtr *application) rateLimit(next http.Handler) http.Handler {
	//code here will only run once, the first time this function is called.'
	//this code is universal in our server, all requests entering our server
	//will all share the logic dictated here	

	type clientInfo struct{
		limiterPtr *rate.Limiter
		lastSeen time.Time
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
					if time.Since(clientInfo.lastSeen) > 3 * time.Minute {
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
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				appPtr.serverErrorResponse(w, r, err)
				return
			}

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
*/
