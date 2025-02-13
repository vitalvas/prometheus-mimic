package gateway

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestBasicAuthMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		users          []User
		authHeader     string
		expectedStatus int
		expectedUser   *User
	}{
		{
			name:           "No users configured",
			users:          nil,
			authHeader:     "",
			expectedStatus: http.StatusOK,
			expectedUser:   &User{},
		},
		{
			name:           "No auth header",
			users:          []User{{Login: "user1", Password: "pass1"}},
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectedUser:   nil,
		},
		{
			name:           "Invalid auth header",
			users:          []User{{Login: "user1", Password: "pass1"}},
			authHeader:     "Invalid header",
			expectedStatus: http.StatusUnauthorized,
			expectedUser:   nil,
		},
		{
			name:           "Invalid base64 encoding",
			users:          []User{{Login: "user1", Password: "pass1"}},
			authHeader:     "Basic invalidbase64",
			expectedStatus: http.StatusUnauthorized,
			expectedUser:   nil,
		},
		{
			name:           "Invalid user credentials",
			users:          []User{{Login: "user1", Password: "pass1"}},
			authHeader:     "Basic " + base64.StdEncoding.EncodeToString([]byte("user1:wrongpass")),
			expectedStatus: http.StatusUnauthorized,
			expectedUser:   nil,
		},
		{
			name:           "Valid user credentials",
			users:          []User{{Login: "user1", Password: "pass1"}},
			authHeader:     "Basic " + base64.StdEncoding.EncodeToString([]byte("user1:pass1")),
			expectedStatus: http.StatusOK,
			expectedUser:   &User{Login: "user1", Password: "pass1"},
		},
		{
			name:           "Multiple users",
			users:          []User{{Login: "user1", Password: "pass1"}, {Login: "user2", Password: "pass2"}},
			authHeader:     "Basic " + base64.StdEncoding.EncodeToString([]byte("user2:pass2")),
			expectedStatus: http.StatusOK,
			expectedUser:   &User{Login: "user2", Password: "pass2"},
		},
		{
			name:           "Valid user credentials in lowercase",
			users:          []User{{Login: "user1", Password: "pass1"}},
			authHeader:     "basic " + base64.StdEncoding.EncodeToString([]byte("user1:pass1")),
			expectedStatus: http.StatusOK,
			expectedUser:   &User{Login: "user1", Password: "pass1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &Gateway{config: &Config{Users: tt.users}}
			router := gin.New()
			router.Use(g.basicAuthMiddleware())
			router.GET("/test", func(c *gin.Context) {
				user, _ := c.Get("user")
				c.JSON(http.StatusOK, user)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedUser != nil {
				var user User
				err := json.Unmarshal(w.Body.Bytes(), &user)
				assert.NoError(t, err)
				assert.Equal(t, *tt.expectedUser, user)
			}
		})
	}
}
