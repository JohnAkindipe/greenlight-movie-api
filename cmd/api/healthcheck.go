package main

import (
	"fmt"
	"net/http"
)

/*********************************************************************************************************************/
/*
The important thing to point out here is that healthcheckHandler is implemented as a method on our application struct.
This is an effective and idiomatic way to make dependencies available to our handlers without resorting to
global variables or closures — any dependency that the healthcheckHandler needs can simply be included as a field
in the application struct when we initialize it in main().

HANDLES GET /v1/healthcheck
*/
func (appPtr *application) healthcheckHandler (w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "status: available")
	fmt.Fprintf(w, "version: %s\n", version)
	fmt.Fprintf(w, "environment: %s\n", appPtr.config.env)
}