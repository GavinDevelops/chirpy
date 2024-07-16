package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
)

type DB struct {
	path string
	mux  *sync.RWMutex
}

type DBStructure struct {
	Chirps map[int]Chirp `json:"chirps"`
	Users  map[int]User  `json:"users"`
}

type User struct {
	Id    int    `json:"id"`
	Email string `json:"email"`
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

func (db *DB) CreateUser(email string) (User, error) {
	loadedDb, loadErr := db.loadDB()
	if loadErr != nil {
		return User{}, loadErr
	}
	offByOne := len(loadedDb.Users) + 1
	loadedDb.Users[offByOne] = User{Id: offByOne, Email: email}
	writeErr := db.writeDB(loadedDb)
	if writeErr != nil {
		return User{}, writeErr
	}
	return loadedDb.Users[offByOne], nil
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
