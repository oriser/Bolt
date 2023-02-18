package run

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/caarlos0/env/v6"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/jmoiron/sqlx"
	slack2 "github.com/oriser/bolt/bot/slack"
	"github.com/oriser/bolt/service"
	"github.com/oriser/bolt/storage/combined"
	db2 "github.com/oriser/bolt/storage/db"
	"github.com/oriser/bolt/storage/slack"
)

type Config struct {
	Bot        slack2.Config
	Handler    service.Config
	SlackSore  slack.Config
	DBLocation string `env:"DB_LOCATION" envDefault:"/var/sqlite/store.db"`
}

func (c Config) String() string {
	res, _ := json.Marshal(&c)
	return string(res)
}

func Run() error {
	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	log.Printf("Starting with options: %s\n", cfg.String())

	slackClient := slack2.NewClient(cfg.Bot)
	id, err := slackClient.GetSelfID()
	if err != nil {
		return fmt.Errorf("get bot self ID: %w", err)
	}

	slackStorage := slack.New(cfg.SlackSore)

	db, err := sqlx.Connect("sqlite3", cfg.DBLocation)
	if err != nil {
		return fmt.Errorf("connect DB: %w", err)
	}
	driver, err := sqlite3.WithInstance(db.DB, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("new sqlite3 migration driver: %w", err)
	}
	dbStorage, err := db2.New(db, driver, "")
	if err != nil {
		return fmt.Errorf("new dbStorage: %w", err)
	}

	serviceHandler, err := service.New(cfg.Handler, combined.NewPrioritizedUserStore(dbStorage, slackStorage), dbStorage, dbStorage, id, slackClient)
	if err != nil {
		return fmt.Errorf("new service: %w", err)
	}

	slackBot := slackClient.ServiceBot(serviceHandler)
	if err := slackBot.ListenAndServe(context.Background()); err != nil {
		return fmt.Errorf("ListenAndServe: %w", err)
	}

	return nil
}
