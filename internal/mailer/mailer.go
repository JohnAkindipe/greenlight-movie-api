package mailer

import (
	"bytes"
	"embed"
	"html/template"
	"time"

	"github.com/go-mail/mail/v2"
)

/*********************************************************************************************************************/
/*
EMBED DIRECTIVE
Below we declare a new variable with the type embed.FS (embedded file system) to hold
our email templates. This has a comment directive in the format `//go:embed <path>`
IMMEDIATELY ABOVE it, which indicates to Go that we want to store the contents of the
./templates directory in the templateFS embedded file system variable.
↓↓↓
*/

//go:embed "templates"
var templateFS embed.FS


// Define a Mailer struct which contains a mail.Dialer instance (used to connect to a
// SMTP server) and the sender information for your emails (the name and address you
// want the email to be from, such as "Alice Smith <alice@example.com>").
type Mailer struct {
    dialerPtr *mail.Dialer
    sender string
}

func New(host string, port int, username, password, sender string) Mailer {

    // Initialize a new mail.Dialer instance with the given SMTP server settings. We 
    // also configure this to use a 5-second timeout whenever we send an email.
	dialerPtr := mail.NewDialer(host, port, username, password)
	dialerPtr.Timeout = 5 * time.Second

	mailer := Mailer{
		dialerPtr: dialerPtr,
		sender: sender,
	}

	return mailer
}

func (mailer Mailer) Send(recipient, templateFile string, data any) error {
    // Use the ParseFS() method to parse the required template file from the embedded 
    // file system: templateFS
	tmpl, err := template.New("email").ParseFS(templateFS, "templates/"+templateFile)
	if err != nil {
		return err
	}

    // Execute the named template "subject", passing in the dynamic data and storing the
    // result in a bytes.Buffer variable.
	subjectPtr := new(bytes.Buffer)
	err = tmpl.ExecuteTemplate(subjectPtr, "subject", data)
	if err != nil {
		return err
	}

    // Follow the same pattern above to execute the "plainBody" template and store the result
    // in the plainBody variable.
	plainBodyPtr := new(bytes.Buffer)
	err = tmpl.ExecuteTemplate(plainBodyPtr, "plainBody", data)
	if err != nil {
		return err
	}

    // And likewise with the "htmlBody" template.
	htmlBodyPtr := new(bytes.Buffer)
	err = tmpl.ExecuteTemplate(htmlBodyPtr, "htmlBody", data)
	if err != nil {
		return err
	}

    // Use the mail.NewMessage() function to initialize a new mail.Message instance. 
    // Then we use the SetHeader() method to set the email recipient, sender and subject
    // headers, the SetBody() method to set the plain-text body, and the AddAlternative()
    // method to set the HTML body. It's important to note that AddAlternative() should
    // always be called *after* SetBody().
	messagePtr := mail.NewMessage()
	messagePtr.SetHeader("To", recipient)
	messagePtr.SetHeader("From", mailer.sender)
	messagePtr.SetHeader("Subject", subjectPtr.String())

	messagePtr.SetBody("text/plain", plainBodyPtr.String())
	messagePtr.AddAlternative("text/html", htmlBodyPtr.String())

    // Call the DialAndSend() method on the dialer, passing in the message to send. This
    // opens a connection to the SMTP server, sends the message, then closes the
    // connection. If there is a timeout, it will return a "dial tcp: i/o timeout"
    // error.
    // Try sending the email up to three times before aborting and returning the final 
    // error. We sleep for 500 milliseconds between each attempt.
    for i := 1; i <= 3; i++ {
        err = mailer.dialerPtr.DialAndSend(messagePtr)
        // If we send the email successfully, return nil.
		//
        if nil == err {
            return nil
        }
        // If it didn't work, sleep for a short time and retry.
        time.Sleep(500 * time.Millisecond)
    }

	//We'll only reach here if we try to send the mail
	//3 times and fail.
	return err
}

/*********************************************************************************************************************/
/*
1. NOTES ON EMBEDDING:
- Check Chapter 13.3 Let's Go Further for more.
- You can specify multiple directories and files in one directive. For example: //go:embed "images" "styles/css" 
"favicon.ico".

- The path separator should always be a forward slash, even on Windows machines.
- You can only use the //go:embed directive on global variables at package level, not within functions or methods. 
If you try to use it in a function or method, you’ll get the error "go:embed cannot apply to var inside func" at 
compile time.

- When you use the directive //go:embed "<path>" to create an embedded file system, the path should be relative to 
the source code file containing the directive. So in our case, //go:embed "templates" embeds the contents of the 
directory at internal/mailer/templates.

- The embedded file system is rooted in the directory which contains the //go:embed directive. So, in our case, to 
get the user_welcome.tmpl file we need to retrieve it from templates/user_welcome.tmpl in the embedded file system.

2. IF NIL == ERR
Hint: In the code above we’re using the clause if nil == err to check if the send was successful, rather than 
if err == nil . They’re functionally equivalent, but having nil as the first item in the clause makes it a bit visually 
jarring and less likely to be confused with the far more common if err != nil clause.
*/