package disttrace

import (
	"crypto/rand"
	"strings"
	"time"

	"github.com/gbrlsnchs/jwt/v3"
)

// AuthClaims holds the signed auth info
type AuthClaims struct {
	payload  jwt.Payload
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
}

// GetToken generates a new token with given claims
func GetToken(claims AuthClaims) (token []byte, err error) {

	if secret == nil {
		initAuth()
	}

	now := time.Now()
	claims.payload = jwt.Payload{
		Issuer:         "disttrace",
		Subject:        claims.Username,
		ExpirationTime: jwt.NumericDate(now.Add(time.Hour)),
		IssuedAt:       jwt.NumericDate(now),
	}

	token, err = jwt.Sign(claims, secret)
	if err != nil {
		log.Error("GetToken: Couldn't sign claims, Error: ", err)
	}
	return
}

// VerifyToken verifies a token
func VerifyToken(token []byte) (err error) {

	var payload AuthClaims
	_, err = jwt.Verify(token, secret, &payload)
	if err != nil {
		log.Error("VerifyToken: Error while verifying authorization token, Error: ", err)
	}

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
