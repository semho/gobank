package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/mux"
)

type APIServer struct {
	listenAddr string
	store      Storage
}

func NewApiserver(listenAddr string, store Storage) *APIServer {
	return &APIServer{
		listenAddr: listenAddr,
		store:      store,
	}
}

func (s *APIServer) Run() {
	router := mux.NewRouter()
	router.HandleFunc("/login", makeHttpHandleFunc(s.handleLogin)).Methods("POST")
	router.HandleFunc("/account", makeHttpHandleFunc(s.handleAccount))
	router.HandleFunc("/account/{id}", withJWTAuth(makeHttpHandleFunc(s.handleGetAccountByID), accountNumberCheckCallback(s.store))).Methods("GET")
	router.HandleFunc("/account/{id}", makeHttpHandleFunc(s.handleDeleteAccount)).Methods("DELETE")
	router.HandleFunc("/transfer", withJWTAuth(makeHttpHandleFunc(s.handleTransfer), accountNumberCheckCallback(s.store))).Methods("POST")

	log.Println("JSON API server running on port: ", s.listenAddr)
	http.ListenAndServe(s.listenAddr, router)
}

func (s *APIServer) handleLogin(w http.ResponseWriter, r *http.Request) error {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil
	}

	acc, err := s.store.GetAccountByNumber(int(req.Number))
	if err != nil {
		return err
	}

	if !acc.ValidPassword(req.Password) {
		return fmt.Errorf("not authorized")
	}

	tokenString, err := createJWT(acc)
	if err != nil {
		return err
	}

	resp := LoginResponse{
		Token:  tokenString,
		Number: acc.Number,
	}

	return WriteJSON(w, http.StatusOK, resp)
}

func (s *APIServer) handleAccount(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "GET" {
		return s.handleGetAccount(w, r)
	}

	if r.Method == "POST" {
		return s.handleCreateAccount(w, r)
	}

	return fmt.Errorf("method not allowed %s", r.Method)
}

func (s *APIServer) handleGetAccount(w http.ResponseWriter, r *http.Request) error {
	accounts, err := s.store.GetAccounts()

	if err != nil {
		return err
	}

	return WriteJSON(w, http.StatusOK, accounts)
}

func (s *APIServer) handleGetAccountByID(w http.ResponseWriter, r *http.Request) error {
	id, err := getID(r)
	if err != nil {
		return err
	}

	account, err := s.store.GetAccountByID(id)
	if err != nil {
		return err
	}

	return WriteJSON(w, http.StatusOK, account)
}

func (s *APIServer) handleCreateAccount(w http.ResponseWriter, r *http.Request) error {
	createAccountReq := CreateAccountRequest{}
	if err := json.NewDecoder(r.Body).Decode(&createAccountReq); err != nil {
		return err
	}

	account, err := NewAccount(createAccountReq.FirstName, createAccountReq.LastName, createAccountReq.Password)
	if err != nil {
		return err
	}

	if err := s.store.CreateAccount(account); err != nil {
		return err
	}

	// tokenString, err := createJWT(account)
	// if err != nil {
	// 	return err
	// }

	// fmt.Println("JWT token: ", tokenString)

	return WriteJSON(w, http.StatusOK, account)
}

func (s *APIServer) handleDeleteAccount(w http.ResponseWriter, r *http.Request) error {
	if r.Method != "DELETE" {
		return fmt.Errorf("method not allowed %s", r.Method)
	}

	id, err := getID(r)
	if err != nil {
		return err
	}

	if err := s.store.DeleteAccount(id); err != nil {
		return err
	}

	return WriteJSON(w, http.StatusOK, map[string]int{"deleted": id})
}

func (s *APIServer) handleTransfer(w http.ResponseWriter, r *http.Request) error {

	transferReq := TransferRequest{}
	if err := json.NewDecoder(r.Body).Decode(&transferReq); err != nil {
		return err
	}

	defer r.Body.Close()

	accFrom, err := s.store.GetAccountByNumber(int(transferReq.FromAccountNumber))
	if err != nil {
		return err
	}

	if accFrom.Balance < int64(transferReq.Amount) {
		return fmt.Errorf("the account balance is not enough")
	}

	_, err = s.store.GetAccountByNumber(int(transferReq.ToAccountNumber))
	if err != nil {
		return err
	}

	err = s.store.SendMoney(transferReq.FromAccountNumber, transferReq.ToAccountNumber, transferReq.Amount)
	if err != nil {
		return err
	}

	responseMsg := TransferResponse{
		Message: fmt.Sprintf("Successfully transferred %d units to account %d", transferReq.Amount, transferReq.ToAccountNumber),
	}

	return WriteJSON(w, http.StatusOK, responseMsg)
}

func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)

	return json.NewEncoder(w).Encode(v)
}

func withJWTAuth(handleFunc http.HandlerFunc, callback AuthCallback) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("calling JWT auth middleware")

		tokenString := r.Header.Get("x-jwt-token")
		token, err := validateJWT(tokenString)

		if err != nil {
			permissionDenied(w)
			return
		}

		if callback != nil {
			if err := callback(r, token); err != nil {
				permissionDenied(w)
				return
			}
		}

		handleFunc(w, r)
	}
}

type AuthCallback func(r *http.Request, token *jwt.Token) error

// Калбэк, который проверяет номер счета аккаунта из БД
func accountNumberCheckCallback(s Storage) AuthCallback {
	return func(r *http.Request, token *jwt.Token) error {
		account := &Account{}
		requestBodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			return err
		}
		if r.Method == "GET" {

			userID, err := getID(r)
			if err != nil {
				return err
			}

			account, err = s.GetAccountByID(userID)
			if err != nil {
				return err
			}

		}

		if r.Method == "POST" {

			transfer := TransferRequest{}
			err := json.Unmarshal(requestBodyBytes, &transfer)
			if err != nil {
				return err
			}
			account.Number = int64(transfer.FromAccountNumber)

		}

		claims := token.Claims.(jwt.MapClaims)
		if account.Number != int64(claims["accountNumber"].(float64)) {
			return errors.New("account number mismatch")
		}

		r.Body = io.NopCloser(bytes.NewReader(requestBodyBytes))

		return nil
	}
}

func permissionDenied(w http.ResponseWriter) {
	WriteJSON(w, http.StatusForbidden, ApiError{Error: "permission denied"})
}

// TODO:для добавления в переменные окружения в консоль вводим export JWT_SECRET=mysecret, в рабочей версии должен находиться в энвах .env
func validateJWT(toketString string) (*jwt.Token, error) {
	secret := os.Getenv("JWT_SECRET")
	return jwt.Parse(toketString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(secret), nil
	})
}

func createJWT(acc *Account) (string, error) {
	claim := &jwt.MapClaims{
		"expiresAt":     15000,
		"accountNumber": acc.Number,
	}

	secret := os.Getenv("JWT_SECRET")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claim)

	return token.SignedString([]byte(secret))
}

type apiFunc func(http.ResponseWriter, *http.Request) error

type ApiError struct {
	Error string `json:"error"`
}

func makeHttpHandleFunc(f apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			WriteJSON(w, http.StatusBadRequest, ApiError{Error: err.Error()})
		}
	}
}

func getID(r *http.Request) (int, error) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return id, fmt.Errorf("invalid id given %s", idStr)
	}

	return id, nil
}
