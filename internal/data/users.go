package data

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"greenlight-movie-api/internal/validator"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Define a custom ErrDuplicateEmail error.
var (
	ErrDuplicateEmail = errors.New("duplicate email")
	ErrInvalidToken   = errors.New("invalid token")
	ErrExpiredToken   = errors.New("expired token")
)

// Define a User struct to represent an individual user. Importantly, notice how we are
// using the json:"-" struct tag to prevent the Password and Version fields appearing in
// any output when we encode it to JSON. Also notice that the Password field uses the
// custom password type defined below.
type User struct {
	ID         int64     `json:"id"`
	Created_At time.Time `json:"created_at"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	Password   password  `json:"-"`
	Activated  bool      `json:"activated"`
	Version    int       `json:"-"`
}

var AnonymousUser = &User{}

// Check if a User instance is the AnonymousUser.
// In essence, this will check if the pointer we receive
// points to the same address in memory as the AnonymousUser
func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}

// USER MODEL
// Create a UserModel struct which wraps the connection pool.
type UserModel struct {
	DBPtr *sql.DB
}

/*********************************************************************************************************************/
// Create a custom password type which is a struct containing the plaintext and hashed
// versions of the password for a user. The plaintext field is a *pointer* to a string,
// so that we're able to distinguish between a plaintext password not being present in
// the struct at all, versus a plaintext password which is the empty string "".
type password struct {
	plaintext *string
	hash      []byte
}

/*********************************************************************************************************************/
// Additionally, we’re going to want to use the email and plaintext password validation checks again independently
// later, so we’ll define those checks in some standalone functions.

// VALIDATE EMAIL
// Check that the Email field is not the empty string, and that it matches the regular expression for email addresses
// that we added in our validator package.
// This function will appear as data.ValidateEmail in functions that call it outside the "data" package.
func ValidateEmail(validatorPtr *validator.Validator, email string) {
	validatorPtr.Check(
		email != "",
		"email",
		"cannot be empty",
	)
	validatorPtr.Check(
		validator.Matches(email, validator.EmailRX),
		"email",
		"is invalid",
	)
}

/*********************************************************************************************************************/
//VALIDATE PLAINTEXT PASSWORD
// If the Password.plaintext field is not nil, then check that the value is not the empty string and is between 8 and
// 72 bytes long.
func ValidatePlaintextPassword(validatorPtr *validator.Validator, plaintextPswrd string) {
	validatorPtr.Check(
		plaintextPswrd != "",
		"password",
		"cannot be empty",
	)

	validatorPtr.Check(
		len(plaintextPswrd) >= 8,
		"password",
		"cannot be less than 8 bytes",
	)

	validatorPtr.Check(
		len(plaintextPswrd) <= 72,
		"password",
		"cannot be greater than 72 bytes",
	)
}

/*********************************************************************************************************************/
/*
VALIDATE USER
*/
func ValidateUser(validatorPtr *validator.Validator, userPtr *User) {
	//VALIDATE NAME
	//Check that the Name field is not the empty string, and the value is less than 500 bytes long.
	validatorPtr.Check(
		userPtr.Name != "",
		"name",
		"cannot be empty",
	)
	validatorPtr.Check(
		len(userPtr.Name) <= 500,
		"name",
		"cannot be more than 500 bytes long",
	)

	// VALIDATE EMAIL
	ValidateEmail(validatorPtr, userPtr.Email)

	//VALIDATE PLAINTEXT PASSWORD
	//We should check if the plaintext which is a *string is nil
	//before passing it to the validate function to prevent a
	//nil pointer dereference error.
	if userPtr.Password.plaintext != nil {
		ValidatePlaintextPassword(validatorPtr, *userPtr.Password.plaintext)
	}

	//VALIDATE PASSWORD HASH
	// If the password hash is ever nil, this will be due to a logic error in our
	// codebase (probably because we forgot to set a password for the user). It's a
	// useful sanity check to include here, but it's not a problem with the data
	// provided by the client. So rather than adding an error to the validation map we
	// raise a panic instead.
	if userPtr.Password.hash == nil {
		panic("missing password hash for user.")
	}
}

/*********************************************************************************************************************/
// The Set() method calculates the bcrypt hash of a plaintext password, and stores both
// the hash and the plaintext versions in the struct. It returns an error if there was
// an error encountered while hashing the passsword
func (passwordPtr *password) Set(plaintextPswrd string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPswrd), 12)
	if err != nil {
		return err
	}
	passwordPtr.plaintext = &plaintextPswrd
	passwordPtr.hash = hash
	return nil
}

// The Matches() method checks whether the provided plaintext password matches the
// hashed password stored in the struct, returning true if it matches and false
// otherwise. If the error returned is a mismatch error we return nil as the error
// value, otherwise we return false as well as the error value.
func (passwordPtr *password) Matches(givenPswrd string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(passwordPtr.hash, []byte(givenPswrd))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}
	// passwordPtr.plaintext = &givenPswrd
	return true, nil
}

/*********************************************************************************************************************/
/*
USER MODEL DB INTERACTIONS (CRUD)
*/
/*
CREATE (INSERT) USER - Insert a new record in the database for the user. Note that the id, created_at and
version fields are all automatically generated by our database, so we use the
RETURNING clause to read them into the User struct after the insert, in the same way
that we did when creating a movie.
*/
func (userModel UserModel) InsertUser(userPtr *User) error {
	ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelFunc()

	rowPtr := userModel.DBPtr.QueryRowContext(
		ctx,
		`
		INSERT INTO users(name, email, password_hash, activated)
		VALUES($1, $2, $3, $4) RETURNING id, created_at, version
	`, userPtr.Name, userPtr.Email, userPtr.Password.hash, userPtr.Activated)

	// If the table already contains a record with this email address, then when we try
	// to perform the insert there will be a violation of the UNIQUE "users_email_key"
	// constraint that we set up in the previous chapter. We check for this error
	// specifically, and return custom ErrDuplicateEmail error instead.
	err := rowPtr.Scan(&userPtr.ID, &userPtr.Created_At, &userPtr.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail
		default:
			return err
		}
	}
	return nil
}

/*
READ (GET) USER (Named GetByEmail by Author) Retrieve the User details from the database
based on the user's email address. Because we have a UNIQUE constraint on the email column,
this SQL query will only return one record (or none at all, in which case we return a
ErrRecordNotFound error).
*/
func (userModel UserModel) GetUserByEmail(email string) (*User, error) {
	// Create a user variable where we will copy the result of
	// the db query into.
	var user User
	query := `
		SELECT * FROM users WHERE email = $1
	`

	ctx, cancelFunc := context.WithTimeout(context.Background(), (3 * time.Second))
	defer cancelFunc()

	rowPtr := userModel.DBPtr.QueryRowContext(
		ctx,
		query,
		email,
	)
	// scan the response data into the fields of the User struct.
	err := rowPtr.Scan(
		&user.ID,
		&user.Created_At,
		&user.Name,
		&user.Email,
		&user.Password.hash,
		&user.Activated,
		&user.Version,
	)

	// Handle any errors. If there was no matching user found, Scan() will return
	// a sql.ErrNoRows error. We check for this and return our custom ErrRecordNotFound
	// error instead.
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &user, nil
}

func (userModel UserModel) GetUserByID(userID int64) (*User, error) {
	// Create a user variable where we will copy the result of
	// the db query into.
	var user User
	query := `
		SELECT * FROM users WHERE id = $1
	`

	ctx, cancelFunc := context.WithTimeout(context.Background(), (3 * time.Second))
	defer cancelFunc()

	rowPtr := userModel.DBPtr.QueryRowContext(
		ctx,
		query,
		userID,
	)
	// scan the response data into the fields of the User struct.
	err := rowPtr.Scan(
		&user.ID,
		&user.Created_At,
		&user.Name,
		&user.Email,
		&user.Password.hash,
		&user.Activated,
		&user.Version,
	)

	// Handle any errors. If there was no matching user found, Scan() will return
	// a sql.ErrNoRows error. We check for this and return our custom ErrRecordNotFound
	// error instead.
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &user, nil
}

/*
UPDATE USER - Update the details for a specific user. Notice that we check against the version
field to help prevent any race conditions during the request cycle, just like we did
when updating a movie. And we also check for a violation of the "users_email_key"
constraint when performing the update, just like we did when inserting the user
record originally.
*/
func (userModel UserModel) UpdateUser(userPtr *User) error {
	//query to update required fields
	//why are we updating the password_hash from here? seems
	//like a security risk.
	query := `
        UPDATE users 
        SET name = $1, email = $2, password_hash = $3, activated = $4, version = version + 1
        WHERE id = $5 AND version = $6
        RETURNING version
	`

	ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelFunc()
	//execute the query with the appropriate arguments, notice that
	//we're also updating the version by 1 from the previuos value
	//this will happen everytime we update a resource
	//Refer notes(1) for notes on optimistic concurrency
	rowPtr := userModel.DBPtr.QueryRowContext(
		ctx,
		query,
		userPtr.Name,
		userPtr.Email,
		userPtr.Password.hash,
		userPtr.Activated,
		userPtr.ID,
		userPtr.Version,
	)

	//Scan the row into the userPtr and handle any potential errors
	err := rowPtr.Scan(
		&userPtr.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail
		default:
			return err
		}
	}

	return nil
}

/*********************************************************************************************************************/
//GETUSERFORTOKEN
//This function will check the token table for a given token and return the user associated with the token
//Appears we may have a general token table where we store different types of token (session, activation tokens etc)
//the tokenType checks what type of token we want to get for a particular user from our general-purpose token table.
func (userModel UserModel) UpdateUserForToken(tokenHash []byte, tokenType string) (*User, error) {
	var user User

	query := `
        UPDATE users
		SET activated = true, version = version + 1
		FROM tokens
		WHERE tokens.user_id = users.id
		AND tokens.hash = $1 
		AND tokens.scope = $2
		RETURNING users.*
	`
	ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelFunc()

	queryResult := userModel.DBPtr.QueryRowContext(ctx, query, tokenHash, tokenType)
	err := queryResult.Scan(
		&user.ID, &user.Created_At, &user.Name, &user.Email, &user.Password.hash, &user.Activated,
		&user.Version,
	)

	//if there was an error
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	//user has already been updated at this point
	return &user, nil

	// //token exists and has not expired
	// //set activated to true on the user and return all columns
	// query = `
	// 	UPDATE users
	// 	SET activated = true, version = version + 1
	// 	WHERE email = $1
	// 	RETURNING *
	// `
	// ctx, cancelFunc = context.WithTimeout(context.Background(), 3 * time.Second)
	// defer cancelFunc()

	// rowPtr := userModel.DBPtr.QueryRowContext(ctx, query, user.Email)
	// err = rowPtr.Scan(&user.ID, &user.Created_At, &user.Name, &user.Email, &user.Password, &user.Activated, &user.Version)
	// if err != nil { //curious what kind of error we could possibly encounter here?
	// 	return nil, err
	// }

	// //don't feel comfortable returning an error if this particular delete token operation fails
	// //the token exists in the db and has not expired, we shouldn't be returning an error to our
	// //user if our delete operation fails. the user should get an error if the token expired or
	// //doesn't exist in the db. neither is true in this case.
	// err = DeleteToken(userModel.DBPtr, retrievedToken.Hash)
	// if err != nil {
	// 	//log the error or somn
	// 	panic(err) //i think a panic is better in this case; this is an operation we don't
	// 	//expect to fail, if it does we should let the user know we have a sever error.
	// 	//which will be communicated to the client from our recover middleware, when we panic.
	// }
}

/*********************************************************************************************************************/
/*
GETFORTOKEN
Get the user for a specific token, Given the scope of the token and the token's plaintext value.
*/
func (userModel UserModel) GetForToken(tokenScope, tokenPlaintext string) (*User, error) {
	// Calculate the SHA-256 hash of the plaintext token provided by the client.
	// Remember that this returns a byte *array* with length 32, not a slice.
	tokenHash := sha256.Sum256([]byte(tokenPlaintext))
	// Set up the SQL query.
	query := `
        SELECT users.id, users.created_at, users.name, users.email, users.password_hash, users.activated, users.version
        FROM users
        INNER JOIN tokens
        ON users.id = tokens.user_id
        WHERE tokens.hash = $1
        AND tokens.scope = $2 
        AND tokens.expiry > $3`
	// Create a slice containing the query arguments. Notice how we use the [:] operator
	// to get a slice containing the token hash, rather than passing in the array (which
	// is not supported by the pq driver), and that we pass the current time as the
	// value to check against the token expiry.
	args := []any{tokenHash[:], tokenScope, time.Now()}
	var user User
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// Execute the query, scanning the return values into a User struct. If no matching
	// record is found we return an ErrRecordNotFound error.
	err := userModel.DBPtr.QueryRowContext(ctx, query, args...).Scan(
		&user.ID,
		&user.Created_At,
		&user.Name,
		&user.Email,
		&user.Password.hash,
		&user.Activated,
		&user.Version,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	// Return the matching user.
	return &user, nil
}

// delete token from the db
func DeleteToken(dbPtr *sql.DB, tokenHash []byte) error {
	// TODO: maybe this should come before retrieving user from db.
	query := `
		DELETE FROM tokens
		WHERE hash = $1
	`
	ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelFunc()

	//ignore result, handle error
	_, err := dbPtr.ExecContext(ctx, query, tokenHash)
	return err
}
