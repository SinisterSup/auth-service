package utils

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/SinisterSup/auth-service/db"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type JWTClaim struct {
	UserId string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func GenerateToken(userId, email string) (string, error) {
	claims := JWTClaim{
		UserId: userId,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}

func ValidateToken(tokenString string) (*JWTClaim, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaim{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JWTClaim)
	if !ok {
		return nil, errors.New("couldn't parse JWTclaims")
	}
	if claims.ExpiresAt.Time.Before(time.Now()) {
		return nil, errors.New("token expired")
	}
	revoked, err := isTokenRevoked(claims.UserId, tokenString)
	// fmt.Println("token revoked? -", revoked)
	if err != nil {
		return nil, errors.New("error checking token status")
	}
	if revoked {
		return nil, errors.New("token has been revoked")
	}

	return claims, nil
}

func isTokenRevoked(userId string, tokenString string) (bool, error) {
	log.Println("starting validating if token has been revoked")
	objectId, err := primitive.ObjectIDFromHex(userId)
	if err != nil {
		return true, err
	}
	log.Println("has no error while finding objectID")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := db.DB.Collection("users")

    userCount, err := collection.CountDocuments(ctx, bson.M{"_id": objectId})
    log.Println("checking collection.countdocs")
	if err != nil {
        return false, err
    }
	log.Println("no error while counting user docs")
    if userCount == 0 {
        return false, errors.New("user not found")
    }
	log.Println("no error while counting users")

    filter := bson.M{
        "_id": objectId,
        "revoked_tokens": bson.M{
            "$elemMatch": bson.M{
                "token": tokenString,
            },
        },
    }

	count, err := collection.CountDocuments(
		ctx, 
		filter,
	)
    if err != nil {
        return false, fmt.Errorf("database error: %v", err)
    }

	// var user struct {
	// 	RevokedTokens []struct {
	// 		Token string `bson:"token"`
	// 	} `bson:"revoked_tokens"`
	// }

	// err = collection.FindOne(ctx, bson.M{
	// 	"_id": objectId,
	// 	"revoked_tokens": bson.M{
	// 		"$elemMatch": bson.M{
	// 			"token": tokenString,
	// 		},
	// 	},
	// }).Decode(&user)

	// if err.Error() == "mongo: no documents in result" {
	// 	return false, nil
	// }

	log.Printf("the collections.docCount is ? - %v", count)
	return count > 0, nil
}