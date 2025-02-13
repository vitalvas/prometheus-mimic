package gateway

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (g *Gateway) basicAuthMiddleware() gin.HandlerFunc {
	if g.config.Users == nil {
		return func(c *gin.Context) {
			c.Set("user", &User{})
			c.Next()
		}
	}

	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.Header("WWW-Authenticate", `Basic realm="Authorization Required"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		split := strings.SplitN(auth, " ", 2)
		if len(split) != 2 || strings.ToLower(split[0]) != "basic" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		payload, err := base64.StdEncoding.DecodeString(split[1])
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		var authenticatedUser *User
		for _, user := range g.config.Users {
			if user.Login == pair[0] && user.Password == pair[1] {
				authenticatedUser = &user
				break
			}
		}

		if authenticatedUser == nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Set("user", authenticatedUser)
		c.Next()
	}
}
