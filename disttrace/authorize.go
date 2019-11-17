package disttrace

import (
	"crypto/rand"
	"strings"
	"time"

	"github.com/gbrlsnchs/jwt/v3"
)

// AuthClaims holds the signed auth info
type AuthClaims struct {
	Payload  jwt.Payload
	Username string
}

// holds the hashed secret
var secret *jwt.HMACSHA

// initialize the secret
func initAuth() {

	randBytes := make([]byte, 100)
	if _, err := rand.Read(randBytes); err != nil {
		log.Fatal("initJWT: Error while collecting randomness, Error: ", err)
	}

	secret = jwt.NewHS256(randBytes)
	log.Debug("initAuth: Secret for signing of auth tokens generated")
}

// GetToken generates a new token with given claims
func GetToken(claims AuthClaims) (token []byte, err error) {

	log.Debug("GetToken: Generating new auth token...")

	if secret == nil {
		initAuth()
	}

	now := time.Now()
	claims.Payload = jwt.Payload{
		Issuer:         "disttrace",
		Subject:        claims.Username,
		ExpirationTime: jwt.NumericDate(now.Add(time.Hour)),
		IssuedAt:       jwt.NumericDate(now),
	}

	token, err = jwt.Sign(claims, secret)
	if err != nil {
		log.Error("GetToken: Couldn't sign claims, Error: ", err)
	}

	log.Debug("GetToken: Returning new auth token")
	return
}

// VerifyToken verifies a token
func VerifyToken(token []byte) (err error) {

	var payload AuthClaims

	if secret == nil {
		initAuth()
	}

	// Validate claims "iat" and "exp"
	now := time.Now()
	iatValidator := jwt.IssuedAtValidator(now)
	expValidator := jwt.ExpirationTimeValidator(now)
	validateOptions := jwt.ValidatePayload(&payload.Payload, iatValidator, expValidator)

	_, err = jwt.Verify(token, secret, &payload, validateOptions)
	if err != nil {
		log.Error("VerifyToken: Error while verifying authorization auth token, Error: ", err)
	}

	log.Debug("VerifyToken: Successfully verified auth token")
	return
}

// TokenFromAuthHeader extracts the JWT token from the Authorization header
func TokenFromAuthHeader(header string) (token string) {

	splitToken := strings.Split(header, "Bearer ")
	if len(splitToken) > 1 {
		token = splitToken[1]
	}

	return
}
