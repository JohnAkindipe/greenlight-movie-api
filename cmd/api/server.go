package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (appPtr *application) serve() error {

	shutdownErrorCh := make(chan error)
	// SERVER SETUP
	srvPtr := &http.Server{
		Addr:         fmt.Sprintf(":%d", appPtr.config.port),
		Handler:      appPtr.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     slog.NewLogLogger(appPtr.logger.Handler(), slog.LevelError),
	}

    // Start a background goroutine.
    go func() {
        // Create a quit channel which carries os.Signal values.
		// Read Notes(1) for why quit has to be a buffered channel
        quit := make(chan os.Signal, 1)

        // Use signal.Notify() to listen for incoming SIGINT and SIGTERM signals and 
        // relay them to the quit channel. Any other signals will not be caught by
        // signal.Notify() and will retain their default behavior.
        signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

        // Read the signal from the quit channel. This code will block until a signal is
        // received.
        s := <-quit

        // Log a message to say that the signal has been caught. 
		// and we're shutting down the server Notice that we also
        // call the String() method on the signal to get the signal name and include it
        // in the log entry attributes.
        appPtr.logger.Info("shutting down server", "signal", s.String())

	    // Create a context with a 30-second timeout.
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        // Call Shutdown() on our server, passing in the context we just made.
        // Shutdown() will return nil if the graceful shutdown was successful, or an
        // error (which may happen because of a problem closing the listeners, or 
        // because the shutdown didn't complete before the 30-second context deadline is
        // hit). We relay this return value to the shutdownError channel.
		err := srvPtr.Shutdown(ctx); 
		// if err != nil {
		// 	appPtr.logger.Error("shutdown error", "error", err)
		// }

		shutdownErrorCh <- err
    }()

	// SERVER START THE HTTP SERVER
	// log that we're starting the server at this port and in this environment
	appPtr.logger.Info("starting server", "addr", srvPtr.Addr, "env", appPtr.config.env)
	// call the listen and serve method of srvPtr
	err := srvPtr.ListenAndServe(); 

    // Calling Shutdown() on our server will cause ListenAndServe() to immediately 
    // return a http.ErrServerClosed error. So if we see this error, it is actually a
    // good thing and an indication that the graceful shutdown has started. So we check 
    // specifically for this, only returning the error if it is NOT http.ErrServerClosed. 
	if !errors.Is(err, http.ErrServerClosed) {
		appPtr.logger.Error("listen and serve error", "error", err)
		return err
	}

    // Otherwise, we wait to receive the return value from Shutdown() on the  
    // shutdownError channel. If return value is an error, we know that there was a
    // problem with the graceful shutdown and we return the error.
	err = <-shutdownErrorCh
	if err != nil {
		return err
	}

    // At this point we know that the graceful shutdown completed successfully and we 
    // log a "stopped server" message.
    appPtr.logger.Info("stopped server", "addr", srvPtr.Addr)
	return nil

	//In this implementation, it is possible for the server to exit (if listenAndServe returns an error that
	//is not as a result of calling os.Signal) without calling os.Signal, and thus, allowing for graceful
	//shutdown, the pattern I described in notes(2) is from the docs and will NEVER allow the server to exit
	//without calling os.Signal, even if listenAndServe returns for a different reason. Hence, we will not
	//have graceful shutdown if our server exits for any other reason apart from calling os.Signal(SIGINT
	//OR SIGTERM)
}


/*********************************************************************************************************************/
/*
NOTES:
1. QUIT MUST BE A BUFFERED CHANNEL 
We need to use a buffered channel here because signal.Notify() does not wait for a receiver to be available when 
sending a signal to the quit channel. If we had used a regular (non-buffered) channel here instead, a signal could be 
‘missed’ if our quit channel is not ready to receive at the exact moment that the signal is sent. By using a buffered 
channel, we avoid this problem and ensure that we never miss a signal.

2. SYNCHRONIZING OUR SERVER SHUTDOWN WITH SHUTDOWN SIGNAL
It is a beautiful piece of code that synchronizes our server shutdown. We make a channel named "shutdownCh", this
channel is responsible for making sure that our server cannot exit, until we close this channel in the goroutine
where we receive os.Signals on the "quit" channel. The serve function waits at the end, to receive a value from
the shutdownCh channel. It will block on this receive until we send an os.Signal, which the child goroutine we
spawned will intercept, log the signal we got, call the shutdown function, log errors (if any) from shutting down and
finally close the shutdownCh allowing its parent to unblock on the receive from "shutdownCh" (remember a receive
from a closed channel will always go through), and exit.
There are some ways the server could exit.
i) We send an os.Signal, the goroutine which is blocked waiting for an os.Signal from the "quit" channel receives
the signal, logs the signal and calls the shutdown method on the server, the shutdown method may or may not return
an error (It could return an error if the context's deadline which was passed to it has exceeded, or any error 
encountered closing any listeners or connections), if it does return any error, we log them (A side effect of
calling shutdown however is that "all Serve, ServeTLS, ListenandServe and ListenandServeTLS" methods immediately
return an ErrServerClosed. This is why in the documentation example, a check is made on the error returned by
ListenandServe, if this error is a ErrServerClosed, we know it was an error returned during server shutdown and
we do nothing, otherwise, we know it was an error in starting or closing the connection for whatever reason and
log the error). Immediately the ListenandServe method returns in the parent goroutine, we block on the shutdownCh
until the child goroutine closes the channel.

ii) The second way for the server to shutdown would be if the ListenandServe returned an error for whatever reason
that was not triggered by calling Shutdown. In this case, ListenandServe has returned an error quite alright, but
we must still allow graceful shutdown, hence we won't allow the parent goroutine to exit, until we are sure that
the child goroutine (where we have implemented graceful shutdown) has exited, then and only then do we allow the
parent goroutine to exit, thus in this case, the parent goroutine will block indefinitely even though it is no
longer "listening and serving", until we send an os.Signal, receive on the "quit" channel in the child goroutine,
call "shutdown" which will gracefully shutdown the application, log any errors (if any were returned), then close
the "shutdownCh", which will then signal to the parent goroutine (which is blocked on a receive from this
"shutdownCh") that it can exit.

Thus this logic allows us to 
- make sure the parent doesn't exit until the child goroutine (which executes graceful shutdown) has completed
- ensure graceful shutdown if we receive an os.Signal in our application
- ensure graceful shutdown if our application stops "listeningandserving" for any reason besides sending an
os.Signal
- Essentially, our application can't exit regardless, unless we send this os.Signal. That is the final step to
shutting down our server.

iii) Something to note is that the call to shutdown, though it has a context, doesn't wait for the context
to timeout, IT WANTS TO RETURN IMMEDIATELY. However it is watching for in-flight requests, if it
sees any in-flight requests, it will wait, if in-flight requests return before 30seconds, it returns
immediately the last in-flight request returns, however, if any requests last more than 30s, the method will
return immediately after 30s. It will in this case, return an error saying it's context deadline was exceeded.
It could return an error also in a case where there was any error closing any listeners or connections.

I want to take special note of this from the docs:
FROM DOCS
Shutdown gracefully shuts down the server without interrupting any active connections. Shutdown works by first 
closing all open listeners, then closing all idle connections, and then waiting indefinitely for connections 
to return to idle and then shut down. If the provided context expires before the shutdown is complete, 
Shutdown returns the context's error, otherwise it returns any error returned from closing the Server's 
underlying Listener(s).

When Shutdown is called, Serve, ListenAndServe, and ListenAndServeTLS immediately return ErrServerClosed. 
Make sure the program doesn't exit and waits instead for Shutdown to return.

Shutdown does not attempt to close nor wait for hijacked connections such as WebSockets. The caller of Shutdown 
should separately notify such long-lived connections of shutdown and wait for them to close, if desired. See 
Server.RegisterOnShutdown for a way to register shutdown notification functions.
*/