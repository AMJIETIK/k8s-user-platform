package main

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type User struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

var db *pgxpool.Pool

func main() {
	log.Println("Starting application...")

	// ---- env ----
	if os.Getenv("ENV") != "prod" {
		_ = godotenv.Load()
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	// ---- context ----
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ---- database ----
	var err error
	db, err = pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("Database connection error: %v", err)
	}

	if err := db.Ping(ctx); err != nil {
		log.Fatalf("Database ping failed: %v", err)
	}

	log.Println("Successfully connected to database")
	defer db.Close()

	// ---- routes ----
	http.HandleFunc("/users", createUserHandler)
	http.HandleFunc("/users/list", listUsersHandler)
	http.HandleFunc("/users/delete", deleteUserHandler)
	http.HandleFunc("/users/update", updateUserHandler)
	http.Handle("/", http.FileServer(http.Dir("./static")))

	// ---- server ----
	server := &http.Server{
		Addr: ":" + port,
	}

	go func() {
		log.Printf("Server listening on :%s...\n", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// ---- graceful shutdown ----
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}

// ---------- handlers ----------

func createUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var input User
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if input.Name == "" || input.Email == "" {
		http.Error(w, "Name and email are required", http.StatusBadRequest)
		return
	}

	_, err := db.Exec(
		r.Context(),
		"INSERT INTO users (name, email) VALUES ($1, $2)",
		input.Name,
		input.Email,
	)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("User added"))
}

func listUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := db.Query(r.Context(), "SELECT name, email FROM users")
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.Name, &u.Email); err != nil {
			http.Error(w, "Scan error", http.StatusInternalServerError)
			return
		}
		users = append(users, u)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(users)
}

func deleteUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var input struct {
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if input.Email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	_, err := db.Exec(r.Context(), "DELETE FROM users WHERE email = $1", input.Email)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("User deleted"))
}

func updateUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var input struct {
		OldEmail string `json:"oldEmail"`
		NewName  string `json:"name"`
		NewEmail string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if input.OldEmail == "" || input.NewName == "" || input.NewEmail == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	_, err := db.Exec(
		r.Context(),
		"UPDATE users SET name = $1, email = $2 WHERE email = $3",
		input.NewName,
		input.NewEmail,
		input.OldEmail,
	)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("User updated"))
}
