package utils

import (
	"errors"
	"log"
	"log/slog"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt"
)

type claims struct {
	ChatId string `json:"email"`
	jwt.StandardClaims
}

type Token struct {
	Value     string
	ExpiresAt time.Time
}

type JWT struct {
	Key []byte
}

func (j JWT) GenerateJWTForUser(chatId int64) (*Token, error) {
	expTime := time.Now().Add(10000 * time.Minute)

	claims := &claims{
		ChatId: strconv.FormatInt(chatId, 10),
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(j.Key)
	if err != nil {
		return nil, err
	}

	return &Token{Value: tokenString, ExpiresAt: expTime}, nil
}

func (j JWT) ValidateToken(tokenString string) (bool, error) {
	claims := &claims{}

	tkn, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return j.Key, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrSignatureInvalid) {
			log.Println("invalid token signature")
			return false, nil
		}
		return false, err
	}

	return tkn.Valid, nil
}

func (j JWT) GetChatIdFromToken(tokenString string) (*int64, error) {
	claims := &claims{}

	tkn, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return j.Key, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrSignatureInvalid) {
			slog.Error("invalid token signature")
		}
		return nil, err
	}

	if !tkn.Valid {
		log.Println("token is invalid")
		return nil, nil
	}

	chatId, err := strconv.ParseInt(claims.ChatId, 10, 64)
	if err != nil {
		slog.Error("cannot convert chatId to int")
		return nil, err
	}
	return &chatId, nil
}
