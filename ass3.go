package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

// Product структура для представления товара
type Product struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Price       int    `json:"price"`
}

var (
	db          *sql.DB
	redisClient *redis.Client
)

func initDB() {
	var err error
	db, err = sql.Open("postgres", "postgresql://username:password@localhost/products_db?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}
}

func initRedis() {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
}

func getProductFromDB(id int) (Product, error) {
	var product Product
	row := db.QueryRow("SELECT id, name, description, price FROM products WHERE id = $1", id)
	err := row.Scan(&product.ID, &product.Name, &product.Description, &product.Price)
	if err != nil {
		return Product{}, err
	}
	return product, nil
}

func getProductHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id := params["id"]

	// Check Redis cache first
	cachedProduct, err := redisClient.Get(r.Context(), id).Result()
	if err == nil {
		// Cache hit
		var product Product
		json.Unmarshal([]byte(cachedProduct), &product)
		json.NewEncoder(w).Encode(product)
		return
	}

	// Cache miss, fetch from database
	productID := 0
	fmt.Sscanf(id, "%d", &productID)
	product, err := getProductFromDB(productID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Cache the product with expiration time
	productJSON, _ := json.Marshal(product)
	err = redisClient.Set(r.Context(), id, productJSON, 10*time.Minute).Err()
	if err != nil {
		log.Println("Error caching product:", err)
	}

	json.NewEncoder(w).Encode(product)
}

func main() {
	initDB()
	initRedis()

	router := mux.NewRouter()
	router.HandleFunc("/products/{id}", getProductHandler).Methods("GET")

	log.Fatal(http.ListenAndServe(":8080", router))
}

//CREATE TABLE products (
//    id SERIAL PRIMARY KEY,
//    name VARCHAR(255),
//    description TEXT,
//    price INTEGER
//);
//
//INSERT INTO products (name, description, price) VALUES
//('Product 1', 'Description of product 1', 100),
//('Product 2', 'Description of product 2', 200),
//('Product 3', 'Description of product 3', 300);
