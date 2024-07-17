package database

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type DB struct {
	path string
	mux  *sync.RWMutex
}

type DBStructure struct {
	Chirps        map[int]Chirp        `json:"chirps"`
	Users         map[int]User         `json:"users"`
	RefreshTokens map[int]RefreshToken `json:"refresh_token"`
}

type UserReturn struct {
	Id           int     `json:"id"`
	Email        string  `json:"email"`
	Token        *string `json:"token"`
	RefreshToken string  `json:"refresh_token"`
}

type User struct {
	Id       int    `json:"id"`
	Email    string `json:"email"`
	Password []byte `json:"password"`
}

type RefreshToken struct {
	Token string    `json:"refresh_token"`
	Exp   time.Time `json:"expiration"`
}

type Chirp struct {
	Id       int    `json:"id"`
	Body     string `json:"body"`
	AuthorId int    `json:"author_id"`
}

func NewDB(path string) (*DB, error) {
	db := DB{path: path, mux: &sync.RWMutex{}}
	err := db.ensureDB()
	if err != nil {
		return &db, err
	}

	return &db, nil
}

func (db *DB) CreateChirp(body, authorId string) (Chirp, error) {
	loadedDb, err := db.loadDB()
	if err != nil {
		return Chirp{}, err
	}
	userId, convErr := strconv.Atoi(authorId)
	if convErr != nil {
		return Chirp{}, convErr
	}
	offByOne := len(loadedDb.Chirps) + 1
	loadedDb.Chirps[offByOne] = Chirp{Id: offByOne, Body: body, AuthorId: userId}
	writeErr := db.writeDB(loadedDb)
	if writeErr != nil {
		return Chirp{}, writeErr
	}
	return loadedDb.Chirps[offByOne], nil
}

func (db *DB) GetChirps() ([]Chirp, error) {
	loadedDb, loadErr := db.loadDB()
	if loadErr != nil {
		return []Chirp{}, loadErr
	}
	chirps := []Chirp{}
	for _, chirp := range loadedDb.Chirps {
		chirps = append(chirps, chirp)
	}
	return chirps, nil
}

func (db *DB) GetChirp(id int) (Chirp, error) {
	loadedDb, loadErr := db.loadDB()
	if loadErr != nil {
		return Chirp{}, errors.New(fmt.Sprintf("Error getting chirp with id: %v", id))
	}
	if chirp, exists := loadedDb.Chirps[id]; exists {
		return chirp, nil
	}
	return Chirp{}, errors.New(fmt.Sprintf("Error getting chirp with id: %v", id))
}

func (db *DB) CreateUser(email string, password string) (UserReturn, error) {
	loadedDb, loadErr := db.loadDB()
	if loadErr != nil {
		return UserReturn{}, loadErr
	}
	_, userExists, userErr := db.doesEmailExist(email)
	if userErr != nil {
		return UserReturn{}, userErr
	}
	if userExists {
		return UserReturn{}, errors.New("User already exists")
	}
	offByOne := len(loadedDb.Users) + 1
	hashedPassword, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if hashErr != nil {
		return UserReturn{}, hashErr
	}
	loadedDb.Users[offByOne] = User{
		Id:       offByOne,
		Email:    email,
		Password: hashedPassword,
	}
	writeErr := db.writeDB(loadedDb)
	if writeErr != nil {
		return UserReturn{}, writeErr
	}
	return UserReturn{Email: email, Id: offByOne}, nil
}

func (db *DB) VerifyUser(email, password, jwtSecret string, expiresInSeconds int) (UserReturn, error) {
	user, userExists, err := db.doesEmailExist(email)
	if err != nil {
		return UserReturn{}, err
	}
	if !userExists {
		return UserReturn{}, errors.New("User does not exist")
	}
	compareErr := bcrypt.CompareHashAndPassword(user.Password, []byte(password))
	if compareErr != nil {
		return UserReturn{}, compareErr
	}
	signedToken, signingErr := getSignedToken(user.Id, expiresInSeconds, jwtSecret)
	if signingErr != nil {
		return UserReturn{}, signingErr
	}
	refreshToken, refreshTokenErr := db.getValidOrNewRefreshToken(user.Id)
	if refreshTokenErr != nil {
		return UserReturn{}, refreshTokenErr
	}
	return UserReturn{
		Id:           user.Id,
		Email:        user.Email,
		Token:        &signedToken,
		RefreshToken: refreshToken.Token,
	}, nil
}

func getSignedToken(id, expiresInSeconds int, jwtSecret string) (string, error) {
	d, _ := time.ParseDuration("1h")
	if d.Abs().Seconds() > float64(expiresInSeconds) && expiresInSeconds != 0 {
		d = time.Duration(time.Second * time.Duration(expiresInSeconds))
	}
	jwtToken := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		jwt.RegisteredClaims{
			Issuer:    "chirpy",
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(d)),
			Subject:   strconv.Itoa(id),
		},
	)
	signedToken, signingErr := jwtToken.SignedString([]byte(jwtSecret))
	if signingErr != nil {
		return "", signingErr
	}
	return signedToken, nil
}

func (db *DB) GetNewTokenFromRefreshToken(refreshToken, jwtSecret string) (string, error) {
	loadedDb, loadErr := db.loadDB()
	if loadErr != nil {
		return "", loadErr
	}
	for id, token := range loadedDb.RefreshTokens {
		if token.Token == refreshToken && token.Exp.Sub(time.Now()) > 0 {
			return getSignedToken(id, 0, jwtSecret)
		}
	}
	return "", errors.New("Invalid refresh token")
}

func (db *DB) RemoveRefreshToken(refreshToken string) error {
	loadedDb, loadErr := db.loadDB()
	if loadErr != nil {
		return loadErr
	}
	for id, token := range loadedDb.RefreshTokens {
		if token.Token == refreshToken {
			delete(loadedDb.RefreshTokens, id)
			db.writeDB(loadedDb)
			return nil
		}
	}
	return nil
}

func (db *DB) getValidOrNewRefreshToken(userId int) (RefreshToken, error) {
	token, tokenIsValid, validErr := db.getValidRefreshToken(userId)
	if validErr != nil {
		return RefreshToken{}, validErr
	}
	if !tokenIsValid {
		return db.createNewRefreshToken(userId)
	}
	return token, nil
}

func (db *DB) getValidRefreshToken(userId int) (RefreshToken, bool, error) {
	loadedDb, loadErr := db.loadDB()
	if loadErr != nil {
		return RefreshToken{}, false, loadErr
	}
	refreshToken, refreshTokenExists := loadedDb.RefreshTokens[userId]
	if refreshTokenExists && refreshToken.Exp.Sub(time.Now()) > 0 {
		return refreshToken, refreshTokenExists, nil
	}
	return RefreshToken{}, false, loadErr
}

func (db *DB) createNewRefreshToken(userId int) (RefreshToken, error) {
	randBytes := make([]byte, 32)
	_, readErr := rand.Read(randBytes)
	if readErr != nil {
		return RefreshToken{}, readErr
	}
	token := hex.EncodeToString(randBytes)
	expiration := time.Now().Add(time.Hour * 24 * 60)
	loadedDb, loadErr := db.loadDB()
	if loadErr != nil {
		return RefreshToken{}, loadErr
	}
	refreshToken := RefreshToken{Token: token, Exp: expiration}
	loadedDb.RefreshTokens[userId] = refreshToken
	db.writeDB(loadedDb)
	return refreshToken, nil
}

func (db *DB) UpdateUser(id, email, password string) (UserReturn, error) {
	userId, conversionErr := strconv.Atoi(id)
	if conversionErr != nil {
		return UserReturn{}, conversionErr
	}
	loadedDb, loadErr := db.loadDB()
	if loadErr != nil {
		return UserReturn{}, loadErr
	}
	hashedPassword, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if hashErr != nil {
		return UserReturn{}, hashErr
	}
	loadedDb.Users[userId] = User{Id: userId, Email: email, Password: hashedPassword}
	db.writeDB(loadedDb)
	return UserReturn{Email: loadedDb.Users[userId].Email, Id: loadedDb.Users[userId].Id}, nil
}

func (db *DB) doesEmailExist(email string) (User, bool, error) {
	loadedDb, loadErr := db.loadDB()
	if loadErr != nil {
		return User{}, false, loadErr
	}
	for _, user := range loadedDb.Users {
		if user.Email == email {
			return user, true, nil
		}
	}
	return User{}, false, nil
}

func (db *DB) ensureDB() error {
	_, err := os.ReadFile(db.path)
	if err != nil {
		fmt.Println("Creating db")
		db.writeDB(DBStructure{Chirps: make(map[int]Chirp), Users: make(map[int]User), RefreshTokens: make(map[int]RefreshToken)})
	}
	return nil
}

func (db *DB) loadDB() (DBStructure, error) {
	db.mux.Lock()
	defer db.mux.Unlock()
	file, _ := os.ReadFile(db.path)
	dbStructure := DBStructure{}
	err := json.Unmarshal(file, &dbStructure)
	if err != nil {
		return DBStructure{}, errors.New("Error unmarshaling db")
	}
	return dbStructure, nil
}

func (db *DB) writeDB(dbStructure DBStructure) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	data, marshalErr := json.Marshal(dbStructure)
	if marshalErr != nil {
		fmt.Println("Marshal err")
		return marshalErr
	}
	writeErr := os.WriteFile(db.path, data, 0666)
	if writeErr != nil {
		fmt.Println("Write err")
		return writeErr
	}
	return nil
}
