package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

type DB struct {
	path string
	mux  *sync.RWMutex
}

type DBStructure struct {
	Chirps map[int]Chirp `json:"chirps"`
	Users  map[int]User  `json:"users"`
}

type UserReturn struct {
	Id    int    `json:"id"`
	Email string `json:"email"`
}

type User struct {
	Id       int    `json:"id"`
	Email    string `json:"email"`
	Password []byte `json:"password"`
}

type Chirp struct {
	Id   int    `json:"id"`
	Body string `json:"body"`
}

func NewDB(path string) (*DB, error) {
	db := DB{path: path, mux: &sync.RWMutex{}}
	err := db.ensureDB()
	if err != nil {
		return &db, err
	}

	return &db, nil
}

func (db *DB) CreateChirp(body string) (Chirp, error) {
	loadedDb, err := db.loadDB()
	if err != nil {
		return Chirp{}, err
	}
	offByOne := len(loadedDb.Chirps) + 1
	loadedDb.Chirps[offByOne] = Chirp{Id: offByOne, Body: body}
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

func (db *DB) VerifyUser(email, password string) (UserReturn, error) {
	user, userExists, err := db.doesEmailExist(email)
	if err != nil {
		return UserReturn{}, err
	}
	if !userExists {
		return UserReturn{}, errors.New("User does not exist")
	}
	compareErr := bcrypt.CompareHashAndPassword(user.Password, []byte(password))
	if compareErr != nil {
		return UserReturn{}, errors.New("Invalid password")
	}
	return UserReturn{Id: user.Id, Email: user.Email}, nil

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
		db.writeDB(DBStructure{Chirps: make(map[int]Chirp), Users: make(map[int]User)})
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
