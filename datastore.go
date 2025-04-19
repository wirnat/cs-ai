package cs_ai

import (
	"context"
	"encoding/json"
	"errors"
	_ "github.com/lib/pq"
	"time"
)

type DatastoreConfig struct {
	RedisAddress  string
	RedisPassword string
	RedisDB       int
}

type datastore struct {
	redis *redis.Client
}

func NewDatastore(d DatastoreConfig) (*datastore, error) {
	// Inisialisasi Redis
	rdb := redis.NewClient(&redis.Options{
		DB:       d.RedisDB,
		Addr:     d.RedisAddress,
		Password: d.RedisPassword,
	})

	// Balikkan instance
	return &datastore{
		redis: rdb,
	}, nil
}

func (d *datastore) Write(ctx context.Context, key string, data interface{}) error {
	val, _ := json.Marshal(data)
	err := d.redis.Set(ctx, key, val, 3*time.Hour).Err()
	if err != nil {
		return err
	}
	return nil
}

func (d *datastore) Read(key string) (interface{}, error) {
	//ambil data dari redis berdasarkan  key, dan tampilkan juga datanya
	ctx := context.Background()
	val, err := d.redis.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		// Key tidak ditemukan di Redis
		return nil, nil
	} else if err != nil {
		// Error lain saat mengambil data
		return nil, err
	}

	return val, nil
}
