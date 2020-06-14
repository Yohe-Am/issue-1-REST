package rest

import (
	"encoding/json"
	"fmt"
	"github.com/Yohe-Am/issue-1-REST/pkg/services/auth"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/dgrijalva/jwt-go/request"
)

// ParseAuthTokenMiddleware checks  if the attached request has a valid
// authentication token. If valid JWT token found, it'll extract the
// sub, the username in this case and attaches it to the passed request.
// If no token is found, it'll attach an invalid username.
func ParseAuthTokenMiddleware(s *Setup) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, _ := request.ParseFromRequest(r, request.AuthorizationHeaderExtractor,
				func(token *jwt.Token) (interface{}, error) {
					if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
					}
					return s.TokenSigningSecret, nil
				})
			// error from ParseFromRequest is ignored because returns errors for expired
			// requests and other cases that have nothing to do with parsing.
			if token != nil {
				claimMap, _ := token.Claims.(jwt.MapClaims)
				isInBlacklist, _ := s.AuthService.IsInBlacklist(r.Header.Get("Authorization"))
				switch {
				case isInBlacklist:
					// has logged out token
					break
				case token.Valid:
					// if valid and not expired
					username := claimMap["sub"]
					r.Header.Set("authorized_username", username.(string))
					r.Header.Del("authorized_username_expired")
					next.ServeHTTP(w, r)
					return
				case claimMap.VerifyExpiresAt(time.Now().Add(-s.TokenRefreshLifetime).Unix(), true):
					// if expired but still refreshable
					username := claimMap["sub"]
					r.Header.Set("authorized_username_expired", username.(string))
					r.Header.Del("authorized_username")
					next.ServeHTTP(w, r)
					return
				default:
					// if too expired
					s.Logger.Printf("access with expired token")
				}
			}
			// if not accepted
			r.Header.Set("authorized_username", "---HerUsername25Letters--")
			r.Header.Del("authorized_username_expired")
			next.ServeHTTP(w, r)
		})
	}
}

// CheckForAuthMiddleware blocks access if there's no valid credential's attached
// on the request from the ParseAuthTokenMiddleware.
func CheckForAuthMiddleware(s *Setup) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isAuthenticated(r) {
				next.ServeHTTP(w, r)
				return
			}
			s.Logger.Printf("unauthenticated access attempt")
			addCors(w)
			w.WriteHeader(http.StatusUnauthorized)
		})
	}
}

func isAuthenticated(r *http.Request) bool {
	authUsername := r.Header.Get("authorized_username")
	if authUsername != "" && len(authUsername) < 25 {
		return true
	}
	return false
}

// postTokenAuth returns a handler for POST /token-auth requests
func postTokenAuth(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response jSendResponse
		statusCode := http.StatusOK
		response.Status = "fail"

		requestUser := new(auth.User)
		err := json.NewDecoder(r.Body).Decode(&requestUser)
		if err != nil {
			response.Data = jSendFailData{
				ErrorReason:  "request format",
				ErrorMessage: `bad request, use format {"username":"username","password":"password"}`,
			}
			s.Logger.Printf("bad auth request")
			statusCode = http.StatusBadRequest
		} else {
			requestUser.Email = "" // remove after email auth is fully implemented
			success, err := s.AuthService.Authenticate(requestUser)
			switch err {
			case nil:
				if success {
					{
						if requestUser.Email != "" {
							// todo email auth
						}
					}
					tokenString, err := s.AuthService.GenerateToken(requestUser.Username)
					if err != nil {
						s.Logger.Printf("token generation failed because: %v", err)
						response.Status = "error"
						response.Message = "server error when authenticating"
						statusCode = http.StatusInternalServerError
					} else {
						response.Status = "success"
						var responseData struct {
							Data string `json:"token"`
						}
						responseData.Data = tokenString
						response.Data = responseData
						s.Logger.Printf("user %s got token", requestUser.Username)
					}
				} else {
					s.Logger.Printf("unsuccessful authentication attempt on nonexisting user")
					response.Data = jSendFailData{
						ErrorReason:  "credentials",
						ErrorMessage: "incorrect username or password",
					}
					statusCode = http.StatusUnauthorized
				}
			case auth.ErrUserNotFound:
				s.Logger.Printf("unsuccessful authentication attempt")
				response.Data = jSendFailData{
					ErrorReason:  "credentials",
					ErrorMessage: "incorrect username or password",
				}
				statusCode = http.StatusUnauthorized
			default:
				s.Logger.Printf("auth failed because: %v", err)
				response.Status = "error"
				response.Message = "server error when generating token"
				statusCode = http.StatusInternalServerError
			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// getTokenAuthRefresh returns a handler for GET /token-auth-refresh requests
func getTokenAuthRefresh(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response jSendResponse
		statusCode := http.StatusOK
		response.Status = "fail"
		{ // this block secures the route
			if isAuthenticated(r) {
				// pass, all is good
			} else if r.Header.Get("authorized_username_expired") != "" {
				r.Header.Set("authorized_username", r.Header.Get("authorized_username_expired"))
			} else {
				s.Logger.Printf("unauthorized refresh request")
				addCors(w)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		tokenString, err := s.AuthService.GenerateToken(r.Header.Get("authorized_username"))
		if err != nil {
			s.Logger.Printf("token generation failed because: %v", err)
			response.Status = "error"
			response.Message = "server error when authenticating"
			statusCode = http.StatusInternalServerError
		} else {
			response.Status = "success"
			var responseData struct {
				Data string `json:"token"`
			}
			responseData.Data = tokenString
			response.Data = responseData
			s.Logger.Printf("user %s refreshed token", r.Header.Get("authorized_username"))
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// getLogout returns a handler for GET /logout requests
func getLogout(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response jSendResponse
		statusCode := http.StatusOK
		response.Status = "fail"

		err := invalidateAttachedToken(r, s)
		if err != nil {
			s.Logger.Printf("logout failed because: %v", err)
			response.Status = "error"
			response.Message = "server error when logging out"
			statusCode = http.StatusInternalServerError
		} else {
			response.Status = "success"
			s.Logger.Printf("token was invalidated")
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// invalidateAttachedToken is a helper function.
func invalidateAttachedToken(req *http.Request, s *Setup) error {
	tokenString := req.Header.Get("Authorization")
	return s.AuthService.AddToBlacklist(tokenString)
}
