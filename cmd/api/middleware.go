package main

import (
	"fmt"
	"net/http"
)

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
*/
