package main

import (
	"errors"
	"fmt"
	"greenlight-movie-api/internal/data"
	"greenlight-movie-api/internal/validator"
	"net/http"
	"strconv"
	"time"

	"github.com/pascaldekloe/jwt"
)

func (appPtr *application) createActivationTokenHandler(w http.ResponseWriter, r *http.Request) {
	//Define a struct that describes the data
	//we expect for a registered user looking
	//to get the activation token
	var reqInput struct {
		Email string `json:"email"`
	}

	//decode the json data from the body into the user struct
	err := appPtr.readJSON(w, r, &reqInput)
	if err != nil {
		appPtr.badRequestResponse(w, r, err)
		return
	}

	//Validate the email
	emailValidatorPtr := validator.New()
	data.ValidateEmail(emailValidatorPtr, reqInput.Email)

	//Send Error if email is not valid
	if !emailValidatorPtr.Valid() {
		appPtr.failedValidationResponse(w, r, emailValidatorPtr.Errors)
		return
	}

	//Check if email belongs to a user in our db
	//Send error if no such email in db
	userPtr, err := appPtr.dbModel.UserModel.GetUserByEmail(reqInput.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			emailValidatorPtr.AddError("email", "no matching email address found")
			appPtr.failedValidationResponse(w, r, emailValidatorPtr.Errors)
		default:
			appPtr.serverErrorResponse(w, r, err)
		}
		return
	}

	//EMAIL IS VALID and belongs to a user in our db atp
	//If user is already activated, let themm know they've been activated
	//return an error
	if userPtr.Activated {
		emailValidatorPtr.AddError("email", "user has already been activated")
		appPtr.failedValidationResponse(w, r, emailValidatorPtr.Errors)
		return
	}

	//Create activation token
	tokenPtr, err := appPtr.dbModel.TokenModel.New(data.ScopeActivation, userPtr.ID, 3*24*time.Hour)
	if err != nil {
		appPtr.serverErrorResponse(w, r, err)
		return
	}

	//Launch a background goroutine to send an email to the user containing the activation token
	appPtr.background(func() {
		fmt.Println(userPtr.Email)
		data := map[string]any{
			"userID":          userPtr.ID,
			"activationToken": tokenPtr.Plaintext,
		}
		// Since email addresses MAY be case sensitive, notice that we are sending this
		// email using the address stored in our database for the user --- not to the
		// reqInput.Email address provided by the client in this request.
		err := appPtr.mailer.Send(
			userPtr.Email,
			"activation_token.tmpl",
			data,
		)
		if err != nil {
			appPtr.logger.Error(err.Error())
			//See notes(3) below for why we log instead
		}
	})
	//then an html reponse to the user
	//Send a 202 Accepted response and confirmation message to the client.
	env := envelope{
		"message": "an email will be sent to you with activation instructions",
	}
	err = appPtr.writeJSON(w, http.StatusAccepted, env, nil)
	if err != nil {
		appPtr.serverErrorResponse(w, r, err)
	}
}

// POST /v1/tokens/authentication
// Authentication Token Generation
// Allow a client to exchange their credentials (email address and password) for a stateful authentication token.
func (appPtr *application) createAuthenticationTokenHandler(w http.ResponseWriter, r *http.Request) {
	//read the email and password from the request using the readJSON helper.
	var reqInput struct {
		Email             string `json:"email"`
		PlaintextPassword string `json:"password"`
	}

	err := appPtr.readJSON(w, r, &reqInput)
	if err != nil {
		appPtr.badRequestResponse(w, r, err)
		return
	}

	//Validate the email and password provided by the user.
	userVPtr := validator.New()
	data.ValidateEmail(userVPtr, reqInput.Email)
	data.ValidatePlaintextPassword(userVPtr, reqInput.PlaintextPassword)
	if !userVPtr.Valid() {
		appPtr.failedValidationResponse(w, r, userVPtr.Errors)
		return
	}

	//lookup the user with the email and password in our database
	userPtr, err := appPtr.dbModel.UserModel.GetUserByEmail(reqInput.Email)
	if err != nil {
		switch err { //no user in our db with such email
		case data.ErrRecordNotFound:
			appPtr.invalidCredentialsResponse(w, r)
		default: //problem looking up user in db
			appPtr.serverErrorResponse(w, r, err)
		}
		return
	}

	//user has been found in our db
	//compare password hash of provided password with
	//password hash returned from db
	matches, err := userPtr.Password.Matches(reqInput.PlaintextPassword)
	if err != nil { //error comparing the password and hash
		appPtr.serverErrorResponse(w, r, err)
		return
	}
	if !matches { //the password and hash don't match; err will actually be nil; check code for matches
		appPtr.invalidCredentialsResponse(w, r)
		return
	}
	//the password and hash match
	//Create a new authentication token and store in the tokens db
	tokenPtr, err := appPtr.dbModel.TokenModel.New(data.ScopeAuthentication, userPtr.ID, 24*time.Hour)
	if err != nil { //error generating token or inserting in db
		appPtr.serverErrorResponse(w, r, err)
		return
	}

	//token successfully generated and inserted in db
	//TODO: Do we send the authentication token in an email? we'll prolly send it in an header
	err = appPtr.writeJSON(w, http.StatusCreated, envelope{"auth-token": tokenPtr}, nil)
	if err != nil {
		appPtr.serverErrorResponse(w, r, err)
	}
}

func (appPtr *application) createJWTAuthenticationTokenHandler(w http.ResponseWriter, r *http.Request) {
	//read the email and password from the request using the readJSON helper.
	var reqInput struct {
		Email             string `json:"email"`
		PlaintextPassword string `json:"password"`
	}

	err := appPtr.readJSON(w, r, &reqInput)
	if err != nil {
		appPtr.badRequestResponse(w, r, err)
		return
	}

	//Validate the email and password provided by the user.
	userVPtr := validator.New()
	data.ValidateEmail(userVPtr, reqInput.Email)
	data.ValidatePlaintextPassword(userVPtr, reqInput.PlaintextPassword)
	if !userVPtr.Valid() {
		appPtr.failedValidationResponse(w, r, userVPtr.Errors)
		return
	}

	//lookup the user with the email and password in our database
	userPtr, err := appPtr.dbModel.UserModel.GetUserByEmail(reqInput.Email)
	if err != nil {
		switch err { //no user in our db with such email
		case data.ErrRecordNotFound:
			appPtr.invalidCredentialsResponse(w, r)
		default: //problem looking up user in db
			appPtr.serverErrorResponse(w, r, err)
		}
		return
	}

	//user has been found in our db
	//compare password hash of provided password with
	//password hash returned from db
	matches, err := userPtr.Password.Matches(reqInput.PlaintextPassword)
	if err != nil { //error comparing the password and hash
		appPtr.serverErrorResponse(w, r, err)
		return
	}
	if !matches { //the password and hash don't match; err will actually be nil; check code for matches
		appPtr.invalidCredentialsResponse(w, r)
		return
	}

	// Create a JWT claims struct containing the user ID as the subject, with an issued
	// time of now and validity window of the next 24 hours. We also set the issuer and
	// audience to a unique identifier for our application.
	var claims jwt.Claims
	claims.Subject = strconv.FormatInt(userPtr.ID, 10)
	claims.Issued = jwt.NewNumericTime(time.Now())
	claims.NotBefore = jwt.NewNumericTime(time.Now())
	claims.Expires = jwt.NewNumericTime(time.Now().Add(24 * time.Hour))
	claims.Issuer = "greenlight.akindipe.john"
	claims.Audiences = []string{"greenlight.akindipe.john"}

	// Sign the JWT claims using the HMAC-SHA256 algorithm and the secret key from the
	// application config. This returns a []byte slice containing the JWT as a base64-
	// encoded string.
	jwtToken, err := claims.HMACSign(jwt.HS256, []byte(appPtr.config.jwt.secret))
	if err != nil {
		appPtr.serverErrorResponse(w, r, err)
	}

	// Convert the []byte slice to a string and return it in a JSON response.
	err = appPtr.writeJSON(w, http.StatusCreated, envelope{"auth-token": string(jwtToken)}, nil)
	if err != nil {
		appPtr.serverErrorResponse(w, r, err)
	}
}

/*********************************************************************************************************************/
/*
NOTES
1 WHY DELETE_ALL_FOR_USER method on TokenModel
If you implement an endpoint like this, it’s important to note that this would allow users to potentially have multiple
valid activation tokens ‘on the go’ at any one time. That’s fine — but you just need to make sure that you delete all
the activation tokens for a user once they’ve successfully activated (not just the token that they used).
*/
