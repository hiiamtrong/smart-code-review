package main

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// User struct without proper documentation
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Global variable (not recommended)
var users []User

func main() {
	router := gin.Default()

	// Missing error handling for routes
	router.GET("/users/:id", getUserByID)
	router.POST("/users", createUser)

	// Hardcoded port
	router.Run(":8080")
}

func getUserByID(c *gin.Context) {
	id := c.Param("id")

	// No error handling for conversion
	userID, _ := strconv.Atoi(id)

	// Linear search in slice (inefficient for large data)
	for _, user := range users {
		if user.ID == userID {
			c.JSON(http.StatusOK, user)
			return
		}
	}

	// Missing proper error response
	c.JSON(404, gin.H{"error": "not found"})
}

func createUser(c *gin.Context) {
	var newUser User

	// No error handling for JSON binding
	c.ShouldBindJSON(&newUser)

	// No input validation
	// No duplicate ID check
	users = append(users, newUser)

	// Missing status code
	c.JSON(200, newUser)
}

// Function with poor error handling
func validateUser(user User) bool {
	if user.Name == "" {
		fmt.Println("Name is empty") // Using fmt.Println instead of proper logging
		return false
	}

	// Poor email validation
	if user.Email == "" {
		return false
	}

	return true
}