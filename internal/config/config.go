package config

import (
	"encoding/json"
	"time"
	"transfers-api/internal/logging"

	"github.com/caarlos0/env/v10"
)

type Config struct {
	Business         BusinessConfig `json:"business"`
	MongoDBConfig    MongoDB        `json:"mongodb"`
	MySQLConfig      MySQL          `json:"mysql"`
	MemcachedConfig  Memcached      `json:"memcached"`
	LocalCacheConfig LocalCache     `json:"LocalCache"`
	RabbitMQConfig   RabbitMQ       `json:"rabbitmq"`
	Auth             AuthConfig     `json:"auth"`
}

type BusinessConfig struct {
	TransferMinAmount int `env:"TRANSFER_MIN_AMOUNT" envDefault:"1" json:"transfer_min_amount"`
}

type MongoDB struct {
	ConnectTimeout          time.Duration `env:"MONGODB_CONNECT_TIMEOUT" envDefault:"10s" json:"connect_timeout"`
	Hostname                string        `env:"MONGODB_HOSTNAME" envDefault:"mongodb" json:"hostname"`
	Port                    int           `env:"MONGODB_PORT" envDefault:"27017" json:"port"`
	Username                string        `env:"MONGODB_USERNAME" envDefault:"root" json:"username"`
	Password                string        `env:"MONGODB_PASSWORD" envDefault:"root" json:"password"`
	Database                string        `env:"MONGODB_DATABASE" envDefault:"transfers-db" json:"database"`
	Collection              string        `env:"MONGODB_COLLECTION" envDefault:"transfers" json:"collection"`
	UsersCollection         string        `env:"MONGODB_USERS_COLLECTION" envDefault:"users" json:"users_collection"`
	RefreshTokensCollection string        `env:"MONGODB_REFRESH_TOKENS_COLLECTION" envDefault:"refresh_tokens" json:"refresh_tokens_collection"`
}

type MySQL struct {
	Hostname string `env:"MYSQL_HOSTNAME" envDefault:"mysql" json:"hostname"`
	Port     int    `env:"MYSQL_PORT" envDefault:"3306" json:"port"`
	Username string `env:"MYSQL_USERNAME" envDefault:"root" json:"username"`
	Password string `env:"MYSQL_PASSWORD" envDefault:"root" json:"password"`
	Database string `env:"MYSQL_DATABASE" envDefault:"transfers-db" json:"database"`
}

type Memcached struct {
	Hostname   string `env:"MEMCACHED_HOSTNAME" envDefault:"memcached" json:"hostname"`
	Port       int    `env:"MEMCACHED_PORT" envDefault:"11211" json:"port"`
	TTLSeconds int    `env:"MEMCACHED_TTL_SECONDS" envDefault:"300" json:"ttl_seconds"`
}

type LocalCache struct {
	TTLSeconds int `env:"LocalCache_TTL_SECONDS" envDefault:"30" json:"ttl_seconds"`
}

type RabbitMQ struct {
	Hostname  string `env:"RABBITMQ_HOSTNAME" envDefault:"rabbitmq" json:"hostname"`
	Port      int    `env:"RABBITMQ_PORT" envDefault:"5672" json:"port"`
	Username  string `env:"RABBITMQ_USERNAME" envDefault:"guest" json:"username"`
	Password  string `env:"RABBITMQ_PASSWORD" envDefault:"guest" json:"password"`
	QueueName string `env:"RABBITMQ_QUEUE_NAME" envDefault:"transfers-events" json:"queue_name"`
}

type AuthConfig struct {
	JWTSecret           string `env:"JWT_SECRET" envDefault:"change-me-in-production" json:"jwt_secret"`
	AccessTokenTTLHours int    `env:"JWT_ACCESS_TTL_HOURS" envDefault:"1" json:"access_token_ttl_hours"`
	RefreshTokenTTLDays int    `env:"JWT_REFRESH_TTL_DAYS" envDefault:"7" json:"refresh_token_ttl_days"`
	UserDBDriver        string `env:"AUTH_USER_DB_DRIVER" envDefault:"mongodb" json:"user_db_driver"`
	// Default admin seeded on first run (skipped if username is empty).
	DefaultAdminUsername string `env:"DEFAULT_ADMIN_USERNAME" envDefault:"admin" json:"-"`
	DefaultAdminEmail    string `env:"DEFAULT_ADMIN_EMAIL" envDefault:"admin@transfers.local" json:"-"`
	DefaultAdminPassword string `env:"DEFAULT_ADMIN_PASSWORD" envDefault:"Admin1234!" json:"-"`
}

func ParseFromEnv() *Config {
	var cfg Config
	for _, nested := range []interface{}{
		&cfg.Business,
		&cfg.MongoDBConfig,
		&cfg.MySQLConfig,
		&cfg.MemcachedConfig,
		&cfg.RabbitMQConfig,
		&cfg.Auth,
	} {
		if err := env.Parse(nested); err != nil {
			logging.Logger.Fatalf("error parsing config: %v", err)
		}
	}
	return &cfg
}

func ParseFromJSON(input []byte) *Config {
	var cfg Config
	if err := json.Unmarshal(input, &cfg); err != nil {
		logging.Logger.Fatalf("error parsing config: %v", err)
	}
	return &cfg
}

func (c *Config) String() string {
	bytes, err := json.Marshal(c)
	if err != nil {
		logging.Logger.Fatalf("error marshaling config: %v", err)
	}
	return string(bytes)
}
