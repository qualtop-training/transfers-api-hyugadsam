package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"
	"transfers-api/internal/config"
	"transfers-api/internal/enums"
	"transfers-api/internal/known_errors"
	"transfers-api/internal/models"
	"transfers-api/internal/services"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// compile-time assertion that UsersMongoDBRepo satisfies UserRepository.
var _ services.UserRepository = &UsersMongoDBRepo{}

// UsersMongoDBRepo implements services.UserRepository on top of MongoDB.
type UsersMongoDBRepo struct {
	users         *mongo.Collection
	refreshTokens *mongo.Collection
}

// --- internal DAOs ---

type userMongoDAO struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	Username     string             `bson:"username"`
	Email        string             `bson:"email"`
	PasswordHash string             `bson:"password_hash"`
	Role         string             `bson:"role"`
	CreatedAt    time.Time          `bson:"created_at"`
}

type refreshTokenMongoDAO struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    string             `bson:"user_id"`
	TokenHash string             `bson:"token_hash"`
	ExpiresAt time.Time          `bson:"expires_at"`
	CreatedAt time.Time          `bson:"created_at"`
}

// NewUsersMongoDBRepository connects to MongoDB and returns a UsersMongoDBRepo.
// It reuses the mongo.Client created from cfg; the unique index on users.email is
// ensured on startup.
func NewUsersMongoDBRepository(cfg config.MongoDB) *UsersMongoDBRepo {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	uri := fmt.Sprintf("mongodb://%s:%d", cfg.Hostname, cfg.Port)
	if cfg.Username != "" && cfg.Password != "" {
		uri = fmt.Sprintf("mongodb://%s:%s@%s:%d/?authSource=admin",
			cfg.Username, cfg.Password, cfg.Hostname, cfg.Port)
	}

	clientOpts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		panic(fmt.Sprintf("UsersMongoDBRepo: connect error: %v", err))
	}

	db := client.Database(cfg.Database)
	usersColl := db.Collection(cfg.UsersCollection)
	tokensColl := db.Collection(cfg.RefreshTokensCollection)

	// Ensure unique index on email.
	idxModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	if _, err := usersColl.Indexes().CreateOne(ctx, idxModel); err != nil {
		// Non-fatal: index may already exist.
		_ = err
	}

	return &UsersMongoDBRepo{
		users:         usersColl,
		refreshTokens: tokensColl,
	}
}

// --- User operations ---

func (r *UsersMongoDBRepo) CreateUser(ctx context.Context, user models.User) (string, error) {
	dao := userMongoDAO{
		Username:     user.Username,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		Role:         user.Role.String(),
		CreatedAt:    user.CreatedAt,
	}

	res, err := r.users.InsertOne(ctx, dao)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return "", fmt.Errorf("user with email %s already exists: %w", user.Email, known_errors.ErrDuplicated)
		}
		return "", fmt.Errorf("inserting user: %w", err)
	}

	return res.InsertedID.(primitive.ObjectID).Hex(), nil
}

func (r *UsersMongoDBRepo) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	var dao userMongoDAO
	err := r.users.FindOne(ctx, bson.M{"email": email}).Decode(&dao)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.User{}, fmt.Errorf("user not found: %w", known_errors.ErrNotFound)
		}
		return models.User{}, fmt.Errorf("querying user by email: %w", err)
	}
	return daoToUser(dao), nil
}

func (r *UsersMongoDBRepo) GetUserByID(ctx context.Context, id string) (models.User, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return models.User{}, fmt.Errorf("invalid user id %s: %w", id, known_errors.ErrBadRequest)
	}

	var dao userMongoDAO
	err = r.users.FindOne(ctx, bson.M{"_id": objID}).Decode(&dao)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.User{}, fmt.Errorf("user not found: %w", known_errors.ErrNotFound)
		}
		return models.User{}, fmt.Errorf("querying user by id: %w", err)
	}
	return daoToUser(dao), nil
}

func daoToUser(dao userMongoDAO) models.User {
	return models.User{
		ID:           dao.ID.Hex(),
		Username:     dao.Username,
		Email:        dao.Email,
		PasswordHash: dao.PasswordHash,
		Role:         enums.ParseRole(dao.Role),
		CreatedAt:    dao.CreatedAt,
	}
}

// --- Refresh token operations ---

func (r *UsersMongoDBRepo) CreateRefreshToken(ctx context.Context, token models.RefreshToken) error {
	dao := refreshTokenMongoDAO{
		UserID:    token.UserID,
		TokenHash: token.TokenHash,
		ExpiresAt: token.ExpiresAt,
		CreatedAt: token.CreatedAt,
	}
	_, err := r.refreshTokens.InsertOne(ctx, dao)
	if err != nil {
		return fmt.Errorf("inserting refresh token: %w", err)
	}
	return nil
}

func (r *UsersMongoDBRepo) GetRefreshTokenByUserID(ctx context.Context, userID string) (models.RefreshToken, error) {
	// Return the most recently created token for the user.
	opts := options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}})
	var dao refreshTokenMongoDAO
	err := r.refreshTokens.FindOne(ctx, bson.M{"user_id": userID}, opts).Decode(&dao)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.RefreshToken{}, fmt.Errorf("refresh token not found: %w", known_errors.ErrNotFound)
		}
		return models.RefreshToken{}, fmt.Errorf("querying refresh token: %w", err)
	}
	return models.RefreshToken{
		ID:        dao.ID.Hex(),
		UserID:    dao.UserID,
		TokenHash: dao.TokenHash,
		ExpiresAt: dao.ExpiresAt,
		CreatedAt: dao.CreatedAt,
	}, nil
}

func (r *UsersMongoDBRepo) DeleteRefreshTokenByUserID(ctx context.Context, userID string) error {
	res, err := r.refreshTokens.DeleteMany(ctx, bson.M{"user_id": userID})
	if err != nil {
		return fmt.Errorf("deleting refresh tokens: %w", err)
	}
	if res.DeletedCount == 0 {
		return fmt.Errorf("no active refresh token for user %s: %w", userID, known_errors.ErrNotFound)
	}
	return nil
}
