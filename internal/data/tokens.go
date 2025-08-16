package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"errors"
	"greenlight-movie-api/internal/validator"
	"time"
)

// Define constants for the token scope.
const (
    ScopeActivation = "activation"
)

type TokenModel struct {
	DBPtr *sql.DB
}

type Token struct {
	Plaintext string
	Hash []byte
	UserID int64
	Expiry time.Time
	Scope string
}
/*********************************************************************************************************************/
//GENERATE TOKEN
//no DB interaction here, hence no need to define it as method on tokenModel
func generateToken(scope string, userID int64, ttl time.Duration) (*Token, error) {
    // Create a Token instance containing the user ID, expiry, and scope information.  
    // Notice that we add the provided ttl (time-to-live) duration parameter to the 
    // current time to get the expiry time?
	token := &Token{
		Expiry: time.Now().Add(ttl),
		Scope: scope,
		UserID: userID,
	}

    // Initialize a zero-valued byte slice with a length of 16 bytes.
	randomBytes := make([]byte, 16)

    // Use the Read() function from the crypto/rand package to fill the byte slice with 
    // random bytes from your operating system's CSPRNG. This will return an error if 
    // the CSPRNG fails to function correctly.
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}

    // Encode the byte slice to a base-32-encoded string and assign it to the token 
    // Plaintext field. This will be the token string that we send to the user in their
    // welcome email. They will look similar to this:
    //
    // Y3QMGX3PJ3WLRL2YRTQGQ6KRHU
    // 
    // Note that by default base-32 strings may be padded at the end with the = 
    // character. We don't need this padding character for the purpose of our tokens, so 
    // we use the WithPadding(base32.NoPadding) method in the line below to omit them.
	token.Plaintext = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)

    // Generate a SHA-256 hash of the plaintext token string. This will be the value 
    // that we store in the `hash` field of our database table. Note that the 
    // sha256.Sum256() function returns an *array* of length 32, so to make it easier to  
    // work with we convert it to a slice using the [:] operator before storing it.
	hash := sha256.Sum256(([]byte(token.Plaintext)))
	token.Hash = hash[:]

	return token, nil
}
//Perform a series of validation checks on a given token. Populate the
//errors field of a validator.Validator if any errors encountered during
//validation.
func ValidateToken(tokenValidator *validator.Validator, tokenPlaintext string) {
	//Check that token has been provided
	tokenValidator.Check(
		tokenPlaintext != "",
		"token",
		"must be provided",
	)
	//Check that token plaintext is exactly 26 bytes long.
	tokenValidator.Check(
		len(tokenPlaintext) == 26,
		"token",
		"must be exactly 26 bytes long",
	)
}

/*********************************************************************************************************************/
/*
FUNCTION TO INSERT TOKENS INTO THE TOKENS TABLE IN THE DB
*/
func (tokenModel TokenModel) Insert(token *Token) error {
	ctx, cancelFunc := context.WithTimeout(context.Background(), 3 * time.Second)
	defer cancelFunc()

	query := `
		INSERT INTO tokens (hash, scope, expiry, user_id)
		VALUES($1, $2, $3, $4)
	`

	_, err := tokenModel.DBPtr.ExecContext(
		ctx, 
		query, 
		token.Hash, 
		token.Scope, 
		token.Expiry, 
		token.UserID,
	)

	return err
}

// The New() method is a shortcut which generates a new Token struct and then inserts the
// data in the tokens table. It calls generateToken and tokenModel.Insert
func (tokenModel TokenModel) New(scope string, userID int64, ttl time.Duration) (*Token, error) {
	tokenPtr, err := generateToken(scope, userID, ttl)
	if err != nil {
		return nil, err
	}

	err = tokenModel.Insert(tokenPtr)
	if err != nil {
		return nil, err
	}

	//token successfully created and inserted in db with no errors
	return tokenPtr, nil
}

//DeleteAllForUser: to delete all tokens with a specific scope for a specific user.
func (tokenModel TokenModel) DeleteAllForUser(scope string, userID int64) error {
	ctx, cancelFunc := context.WithTimeout(context.Background(), 3 * time.Second)
	defer cancelFunc()

	query := `
		DELETE FROM tokens
		WHERE scope = $1
		AND user_id = $2
	`

	_, err := tokenModel.DBPtr.ExecContext(ctx, query, scope, userID)
	return err //err may have a value or be nil
}

//GetToken - This will get a token from our database
//We also need the scope to be sure that not only does
//the token exist in our db, it also has the right scope i.e. is it an activation,
//authentication or password-reset token
func (tokenModel TokenModel) GetToken(tokenPlaintext, scope string) (*Token, error) {
	//Generate the hash of the given tokenPlaintext
	hash := sha256.Sum256(([]byte(tokenPlaintext)))
	tokenHash := hash[:]

	//The token variable which will hold the token data to return
	var token Token

	query := `SELECT * FROM tokens WHERE hash = $1 AND scope = $2`

	ctx, cancelFunc := context.WithTimeout(context.Background(), 3 * time.Second)
	defer cancelFunc()

	rowPtr := tokenModel.DBPtr.QueryRowContext(ctx, query, tokenHash, scope)
	err := rowPtr.Scan(
		&token.Hash, &token.Scope, &token.Expiry, &token.UserID,
	)

	//Handle the error if the token doesn't exist or any other errors
	if err != nil {
		switch {
			case errors.Is(err, sql.ErrNoRows):
				return nil, ErrRecordNotFound
			default:
				return nil, err
		}
	}
	//Token exists in our db.
	token.Plaintext = tokenPlaintext
	return &token, nil
}