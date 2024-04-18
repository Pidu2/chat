package main

import (
	"crypto/sha256"
	"database/sql"
	"log"
	"net/http"

	_ "github.com/mattn/go-sqlite3"

	"github.com/Pidu2/chat/internal/middleware"
	"github.com/gin-gonic/gin"
)

const salt string = "87ba2bc8-4d8c-4fd3-8ae1-e57191811443"

type User struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func main() {
	// OPEN DB CONNECTION AND INIT TABLE
	database, err := sql.Open("sqlite3", "./users.sqlite")
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	statement, err := database.Prepare("CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, username TEXT, password TEXT)")
	if err != nil {
		log.Fatal(err)
	}
	statement.Exec()
	log.Println("Table created or already exists.")

	// INIT GIN
	r := gin.Default()
	// Register Endpoint
	r.POST("/register", func(c *gin.Context) {
		queryPeople(database)
		var newUser User
		if err := c.BindJSON(&newUser); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if !userExists(database, newUser.Username) {
			if insertUser(database, newUser.Username, newUser.Password) {
				c.JSON(http.StatusCreated, gin.H{
					"message": "User registered successfully!",
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Error creating user!",
				})
			}
		} else {
			c.JSON(http.StatusConflict, gin.H{
				"message": "User already exists!",
			})
		}
	})
	// Login Endpoint, returning JWT
	r.POST("/login", func(c *gin.Context) {
		var newUser User
		if err := c.BindJSON(&newUser); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if getUser(database, newUser.Username, newUser.Password) {
			jwt, err := middleware.GenerateJWT(newUser.Username)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Error logging in!",
				})
			}
			c.JSON(http.StatusOK, gin.H{
				"jwt": jwt,
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Error logging in!",
			})
		}
	})
	// Testing-Endpoint to validate the JWT and get the associated username
	r.GET("/validateJWT", middleware.TokenAuthMiddleware(), func(c *gin.Context) {
		username := c.MustGet("username").(string)
		c.JSON(http.StatusOK, gin.H{"username": username})
	})
	r.Run("0.0.0.0:8080")
}

func insertUser(database *sql.DB, username string, password string) bool {
	if userExists(database, username) {
		log.Printf("User %s already exists.", username)
		return false
	}
	h := sha256.New()
	h.Write([]byte(password + salt))
	pw_hash := h.Sum(nil)
	statement, err := database.Prepare("INSERT INTO users (username, password) VALUES (?, ?)")
	if err != nil {
		log.Fatal(err)
		return false
	}
	_, err = statement.Exec(username, pw_hash)
	if err != nil {
		log.Fatal(err)
		return false
	}
	log.Printf("Inserted data: %s %s", username, password)
	return true
}

func getUser(database *sql.DB, username string, password string) bool {
	var username_db, password_db string
	h := sha256.New()
	h.Write([]byte(password + salt))
	pw_hash := h.Sum(nil)
	err := database.QueryRow("SELECT username, password FROM users WHERE username = ?", username).Scan(&username_db, &password_db)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("No person found with username %s", username)
		} else {
			log.Fatal(err)
		}
	}
	if password_db == string(pw_hash) {
		log.Printf("Found user with username %s and password %s", username, password)
		return true
	} else {
		log.Printf("Wrong PW for user with username %s and password %s", username, password)
		return false
	}
}

func userExists(database *sql.DB, username string) bool {
	var id int
	err := database.QueryRow("SELECT id FROM users WHERE username = ?", username).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return false // User does not exist
		}
		log.Fatal(err) // An error occurred during the query
	}
	return true // User exists
}

func queryPeople(database *sql.DB) {
	rows, err := database.Query("SELECT id, username FROM users")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var username string
		err = rows.Scan(&id, &username)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Read data: %d, %s", id, username)
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}
}
