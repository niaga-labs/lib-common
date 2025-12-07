package middleware

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// SQL injection patterns to detect
var sqlInjectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)((\%27)|('))\s*((\%6F)|o|(\%4F))((\%72)|r|(\%52))`),
	regexp.MustCompile(`(?i)\w*((\%27)|(\'))((\%6F)|o|(\%4F))((\%72)|r|(\%52))`),
	regexp.MustCompile(`(?i)((\%27)|(\'))union`),
	regexp.MustCompile(`(?i)exec(\s|\+)+(s|x)p\w+`),
	regexp.MustCompile(`(?i)UNION.*SELECT`),
	regexp.MustCompile(`(?i)SELECT.*FROM`),
	regexp.MustCompile(`(?i)INSERT.*INTO`),
	regexp.MustCompile(`(?i)DELETE.*FROM`),
	regexp.MustCompile(`(?i)DROP.*TABLE`),
	regexp.MustCompile(`(?i)UPDATE.*SET`),
	regexp.MustCompile(`(?i)(;|\-\-|\/\*|\*\/|xp_)`),
}

// XSS patterns to detect
var xssPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`),
	regexp.MustCompile(`(?i)<iframe[^>]*>.*?</iframe>`),
	regexp.MustCompile(`(?i)javascript:`),
	regexp.MustCompile(`(?i)on\w+\s*=`), // onclick, onload, etc.
	regexp.MustCompile(`(?i)<embed[^>]*>`),
	regexp.MustCompile(`(?i)<object[^>]*>`),
}

// InputValidation validates request inputs for malicious patterns
func InputValidation() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check query parameters
		for key, values := range c.Request.URL.Query() {
			for _, value := range values {
				if isMalicious(value) {
					c.JSON(http.StatusBadRequest, gin.H{
						"error":   "Invalid input detected",
						"message": "Request contains potentially malicious content",
						"field":   key,
					})
					c.Abort()
					return
				}
			}
		}

		// Check form data (only for form-urlencoded content type, skip for JSON)
		contentType := c.GetHeader("Content-Type")
		if c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH" {
			// Only parse form data if Content-Type is form-urlencoded or multipart
			if strings.Contains(contentType, "application/x-www-form-urlencoded") ||
				strings.Contains(contentType, "multipart/form-data") {
				c.Request.ParseForm()
				for key, values := range c.Request.PostForm {
					for _, value := range values {
						if isMalicious(value) {
							c.JSON(http.StatusBadRequest, gin.H{
								"error":   "Invalid input detected",
								"message": "Request contains potentially malicious content",
								"field":   key,
							})
							c.Abort()
							return
						}
					}
				}
			}
			// Note: JSON body validation should happen in the handler after binding
		}

		c.Next()
	}
}

// isMalicious checks if input contains malicious patterns
func isMalicious(input string) bool {
	input = strings.ToLower(input)

	// Check for SQL injection
	for _, pattern := range sqlInjectionPatterns {
		if pattern.MatchString(input) {
			return true
		}
	}

	// Check for XSS
	for _, pattern := range xssPatterns {
		if pattern.MatchString(input) {
			return true
		}
	}

	return false
}

// SanitizeInput sanitizes user input (use sparingly, prefer validation)
func SanitizeInput(input string) string {
	// Remove dangerous characters
	input = strings.ReplaceAll(input, "<", "&lt;")
	input = strings.ReplaceAll(input, ">", "&gt;")
	input = strings.ReplaceAll(input, "\"", "&quot;")
	input = strings.ReplaceAll(input, "'", "&#x27;")
	input = strings.ReplaceAll(input, "/", "&#x2F;")

	return input
}
