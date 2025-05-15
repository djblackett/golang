module github.com/djblackett/chirpy

go 1.24.1

replace github.com/djblackett/mystrings v0.0.0 => ../mystrings

require (
	github.com/google/uuid v1.6.0
	github.com/joho/godotenv v1.5.1
	github.com/lib/pq v1.10.9
	golang.org/x/crypto v0.38.0
)

require github.com/golang-jwt/jwt/v5 v5.2.2 // indirect
