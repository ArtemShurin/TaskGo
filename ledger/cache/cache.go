package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var Client *redis.Client

const opTimeout = 200 * time.Millisecond

type Config struct {
	Addr     string
	Password string
	DB       int
}

func ConfigFromEnv() (Config, error) {
	cfg := Config{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASSWORD"),
	}
	if cfg.Addr == "" {
		cfg.Addr = "localhost:6379"
	}
	if v := os.Getenv("REDIS_DB"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return Config{}, fmt.Errorf("invalid REDIS_DB %q: must be a non-negative integer", v)
		}
		cfg.DB = n
	}
	return cfg, nil
}

func NewClient(cfg Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:          cfg.Addr,
		Password:      cfg.Password,
		DB:            cfg.DB,
		DialTimeout:   opTimeout,
		ReadTimeout:   opTimeout,
		WriteTimeout:  opTimeout,
		PoolTimeout:   opTimeout,
		MaxRetries:    -1,
		DialerRetries: 1,
	})
}

func Connect(ctx context.Context, cfg Config) (*redis.Client, error) {
	client := NewClient(cfg)

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping redis %s (db %d): %w", cfg.Addr, cfg.DB, err)
	}

	Client = client
	return client, nil
}

func ConnectFromEnv(ctx context.Context) (Config, error) {
	cfg, err := ConfigFromEnv()
	if err != nil {
		return Config{}, err
	}
	if _, err := Connect(ctx, cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Enabled() bool {
	return Client != nil
}

func GetJSON(ctx context.Context, key string, v any) (bool, error) {
	data, err := Client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("redis get %s: %w", key, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return false, fmt.Errorf("decode cached %s: %w", key, err)
	}
	return true, nil
}

func SetJSON(ctx context.Context, key string, v any, ttl time.Duration) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode %s: %w", key, err)
	}
	if err := Client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("redis set %s: %w", key, err)
	}
	return nil
}

func Delete(ctx context.Context, keys ...string) error {
	if err := Client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("redis del: %w", err)
	}
	return nil
}

func DeleteByPattern(ctx context.Context, pattern string) error {
	iter := Client.Scan(ctx, 0, pattern, 100).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("redis scan %s: %w", pattern, err)
	}
	if len(keys) == 0 {
		return nil
	}
	return Delete(ctx, keys...)
}
