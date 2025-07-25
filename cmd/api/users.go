package main

import (
	"errors"
	"fmt"
	"greenlight-movie-api/internal/data"
	"greenlight-movie-api/internal/validator"
	"net/http"
)

/*
DUMMY USER
{
    "name": "John",
    "email": "john@john.com",
    "password": "akindipe123"
}
*/

//POST /v1/users
//To create a new user
func (appPtr *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	var user data.User
	userRegistered := make(chan bool, 1)
	defer close(userRegistered)
	//Launch a background goroutine to send a welcome email to the user
	//After they have successfully been registered. We only want this
	//email to be sent if they were successfully reigstered. We use a
	//channel to communicate this from the parent to the child goroutine.
	appPtr.background(func() {
		registered := <-userRegistered
		if registered {
			//We have successfully inserted the user into our db.
			//send an email, (handling any errors)
			fmt.Println(user.Email)
			err := appPtr.mailer.Send(user.Email, "user_welcome.tmpl", user)
			if err != nil {
				appPtr.logger.Error(err.Error())
				//See notes(3) below for why we log instead
			}
			return
		}
		//receiving weird timeout errors randomly, also works randomly
		//need to investigate why
		fmt.Println("registration email: not sent")
		// panic("test panic")
	})
	//Define a struct that describes the data
	//we expect for a new user
	type newUserInput struct {
		Name     string	`json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	//initialize a new user as the destination to marshal
	//the incoming json data into
	userInput := newUserInput{}

	//decode the json data from the body into the user struct
	err := appPtr.readJSON(w, r, &userInput)
	if err != nil {
		appPtr.badRequestResponse(w, r, err)
		return
	}

	//copy the newuserinput into a data.User struct which we will pass
	//into the data.ValidateUser function. We have to do this because
	//the data.User struct does not allow us to encode or decode the 
	//password field to and from json. This is necessary for security reasons
	//exactly why is not clear to me yet.
	user = data.User{
		Name: userInput.Name,
		Email: userInput.Email,
		Activated: false,
	}
	if err := user.Password.Set(userInput.Password); err != nil {
		appPtr.serverErrorResponse(w, r, err)
		return
	}

	//perform validation checks on the user using the validation
	//we created already
	userValidatorPtr := validator.New()

	data.ValidateUser(userValidatorPtr, &user)
	if !userValidatorPtr.Valid() {
		appPtr.failedValidationResponse(w, r, userValidatorPtr.Errors)
		return
	}

	//At this point, the user has passed all our validation checks
	//we can pass this user into the database to be inserted into the
	//database
	err = appPtr.dbModel.UserModel.InsertUser(&user)
	if err != nil {
		//user registration unsuccessful, let child goroutine know
		//not to send welcome email
		userRegistered <- false
		switch {
			case errors.Is(err, data.ErrDuplicateEmail):
				userValidatorPtr.AddError("email", "an account exists already with that email")
				appPtr.failedValidationResponse(w, r, userValidatorPtr.Errors)
			default:
				appPtr.serverErrorResponse(w, r, err)
		}
		return
	}
	//Let the child goroutine know that we have successfully registered the user
	userRegistered <- true

	//then an html reponse to the user that we have successfully created the user
	//with the data of the newly created user in json. Send an error response
	//if (for whatever reason), we are unable to send the json response
    // Note that we also change this to send the client a 202 Accepted status code.
    // This status code indicates that the request has been accepted for processing, but 
    // the processing has not been completed.
	err = appPtr.writeJSON(w, http.StatusAccepted, envelope{"user": user}, nil)
	//this feels weird to me, we are sending the client information that there was
	//a server error, whereas the user was successfully created and exists in our
	//database, this error doesn't relate to creating the user, but sending a JSON
	//response to the client.
	if err != nil {
		appPtr.serverErrorResponse(w, r, err)
		return
	}
}

//GET /v1/user/:email
//To get a user by their email
func (appPtr *application) getUser(w http.ResponseWriter, r *http.Request) {

}

/*********************************************************************************************************************/
/*
NOTES:
Additional Information
1. EMAIL CASE-SENSITIVITY
Let’s talk quickly about email addresses case-sensitivity in a bit more detail.

Thanks to the specifications in RFC 2821, the domain part of an email address (username@domain) is case-insensitive. 
This means we can be confident that the real-life user behind alice@example.com is the same person as alice@EXAMPLE.COM.

The username part of an email address may or may not be case-sensitive — it depends on the email provider. Almost every 
major email provider treats the username as case-insensitive, but it is not absolutely guaranteed. All we can say here is 
that the real-life user behind the address alice@example.com is very probably (but not definitely) the same as 
ALICE@example.com.

So, what does this mean for our application?

From a security point of view, we should always store the email address using the exact casing provided by the user during 
registration, and we should send them emails using that exact casing only. If we don’t, there is a risk that emails could 
be delivered to the wrong real-life user. It’s particularly important to be aware of this in any workflows that use email 
for authentication purposes, such as a password-reset workflow.

However, because alice@example.com and ALICE@example.com are very probably the same user, we should generally treat email 
addresses as case-insensitive for comparison purposes.

In our registration workflow, using a case-insensitive comparison prevents users from accidentally (or intentionally) 
registering multiple accounts by just using different casing. And from a user-experience point-of-view, in workflows like 
login, activation or password resets, it’s more forgiving for users if we don’t require them to submit their request with 
exactly the same email casing that they used when registering.

2. USER ENUMERATION
It’s important to be aware that our registration endpoint is vulnerable to user enumeration. For example, if an attacker 
wants to know whether alice@example.com has an account with us, all they need to do is send a request like this:

$ BODY='{"name": "Alice Jones", "email": "alice@example.com", "password": "pa55word"}'
$ curl -d "$BODY" localhost:4000/v1/users
{
    "error": {
        "email": "a user with this email address already exists"
    }
}
And they have the answer right there. We’re explicitly telling the attacker that alice@example.com is already a user.

So, what are the risks of leaking this information?

The first, most obvious, risk relates to user privacy. For services that are sensitive or confidential you probably 
don’t want to make it obvious who has an account. The second risk is that it makes it easier for an attacker to 
compromise a user’s account. Once they know a user’s email address, they can potentially:

Target the user with social engineering or another type of tailored attack.
Search for the email address in leaked password tables, and try those same passwords on our service.
Preventing enumeration attacks typically requires two things:

- Making sure that the response sent to the client is always exactly the same, irrespective of whether a user exists or 
not. Generally, this means changing your response wording to be ambiguous, and notifying the user of any problems in a 
side-channel (such as sending them an email to inform them that they already have an account).

- Making sure that the time taken to send the response is always the same, irrespective of whether a user exists or not. 
In Go, this generally means offloading work to a background goroutine.

Unfortunately, these mitigations tend to increase the complexity of your application and add friction and obscurity to 
your workflows. For all your regular users who are not attackers, they’re a negative from a UX point of view. You have 
to ask: is it worth the trade-off?

There are a few things to think about when answering this question. How important is user privacy in your application? 
How attractive (high-value) is a compromised account to an attacker? How important is it to reduce friction in your 
user workflows? The answers to those questions will vary from project-to-project, and will help form the basis for 
your decision.

It’s worth noting that many big-name services, including Twitter, GitHub and Amazon, don’t prevent user enumeration (at 
least not on their registration pages). I’m not suggesting that this makes it OK — just that those companies have 
decided that the additional friction for the user is worse than the privacy and security risks in their specific case.

3. LOGGING IN BACKGROUND GOROUTINE
We are logging in the background goroutine (as opposed to returning a server error) if there was an error sending the
welcome email. We do this because we know that unlike in the synchronous version, we call the serverErrorResponse and
return immediately, meaning there is no possibility that we call serverErrorResponse again if there was an error sending
an html response back to the user. However, with a goroutine, we would be calling the serverErrorResponse twice (which  
would try to set the response header twice, which is an error) on the same request if we call serverErrorResponse in the 
goroutine in the event that we encounter an error while sending the mail, hence the decision to log the error in the 
goroutine instead, so that we can safely send a serverErrorResponse to the user, should we encounter any error sending
a JSON response to inform them that the user was successfully created.
*/