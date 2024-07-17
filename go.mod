module github.com/GavinDevelops/chirpy

replace github.com/GavinDevelops/chirpy/database v0.0.0 => ./database

require github.com/GavinDevelops/chirpy/database v0.0.0

require (
	github.com/golang-jwt/jwt/v5 v5.2.1 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	golang.org/x/crypto v0.25.0 // indirect
)

go 1.22.5
