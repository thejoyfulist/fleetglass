package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/api"
	"github.com/thejoyfulist/fleetglass/internal/config"
	"github.com/thejoyfulist/fleetglass/internal/money"
	"github.com/thejoyfulist/fleetglass/internal/store"
	"github.com/thejoyfulist/fleetglass/internal/version"
)

func main() {
	path := flag.String("config", "", "JSON config path")
	showVersion := flag.Bool("version", false, "show version")
	flag.Parse()
	if *showVersion {
		fmt.Println(version.String())
		return
	}
	c, e := config.Load(*path)
	if e != nil {
		log.Fatal(e)
	}
	if missing := config.OptionalKeysAbsent(*path); len(missing) > 0 {
		log.Printf("optional config keys available: %s", strings.Join(missing, ", "))
	}
	if e := c.ValidateReadAccess(); e != nil {
		log.Fatal(e)
	}
	if flag.NArg() > 0 && flag.Arg(0) == "backup" {
		if _, err := os.Stat(c.DBPath); err != nil {
			log.Fatal("database unavailable: ", err)
		}
	}
	s, e := store.Open(c.DBPath)
	if e != nil {
		log.Fatal(e)
	}
	defer s.Close()
	if flag.NArg() > 0 {
		if flag.Arg(0) == "backup" && flag.NArg() == 2 {
			if err := s.Backup(flag.Arg(1)); err != nil {
				log.Fatal(err)
			}
			fmt.Println("Backup created")
			return
		}
		if flag.Arg(0) != "host" || flag.NArg() < 2 {
			log.Fatal("usage: fleetglass-server backup <file> | host add|list|revoke|remove")
		}
		hostCommand(s, flag.Args()[1:])
		return
	}
	if c.Token == "" {
		log.Print("legacy ingest token absent; enrolled host tokens only")
	}
	if e := s.Maintain(time.Now()); e != nil {
		log.Fatal(e)
	}
	if c.BackupDir != "" {
		if err := dailyBackup(s, c.BackupDir, c.BackupRetentionDays, time.Now().UTC()); err != nil {
			log.Printf("daily backup failed: %v", err)
		}
	}
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for now := range ticker.C {
			if e := s.Maintain(now); e != nil {
				log.Printf("storage maintenance failed: %v", e)
			}
			if c.BackupDir != "" {
				if err := dailyBackup(s, c.BackupDir, c.BackupRetentionDays, now.UTC()); err != nil {
					log.Printf("daily backup failed: %v", err)
				}
			}
		}
	}()
	log.Printf("listening on %s", c.Listen)
	a := api.New(s, c.Token, c.HostID)
	a.ReadToken = c.ReadToken
	a.AllowedHosts = c.AllowedHosts
	a.MaxSubscribers = c.LiveMaxSubscribers
	a.AIPrices = c.AIPrices
	a.AgentNoEditMinutes = c.AgentNoEditMinutes
	a.AISubscriptions = c.AISubscriptions
	a.ClaudeBudget5H = c.ClaudeBudget5H
	a.ClaudeBudgetWeek = c.ClaudeBudgetWeek
	a.EnergyPricePerKWh = c.EnergyPricePerKWh
	a.EnergyCurrency = c.EnergyCurrency
	a.DisplayCurrency = c.DisplayCurrency
	a.Money = &money.Service{DB: s.DB, Source: c.ExchangeRateSource}
	if c.ExchangeRates != nil {
		a.Money.Static = &money.Snapshot{Date: c.ExchangeRates.Date, Source: "static", Rates: c.ExchangeRates.Rates}
	}
	if err := a.Money.Init(); err != nil {
		log.Fatal(err)
	}
	a.DisplayDefaults = c.DisplayDefaults
	a.Version = version.Version
	a.Commit = version.Commit
	a.UpdateCheckURL = c.UpdateCheckURL
	if c.MCPListen != "" {
		mcp := &http.Server{Addr: c.MCPListen, Handler: a.MCPHandler(c.MCPToken), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
		go func() {
			if err := mcp.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("MCP listener unavailable")
			}
		}()
	}
	if c.StatusListen != "" {
		status := &http.Server{Addr: c.StatusListen, Handler: a.StatusHandler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
		go func() {
			if err := status.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("status listener unavailable")
			}
		}()
	}
	server := &http.Server{Addr: c.Listen, Handler: a.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	log.Fatal(server.ListenAndServe())
}

func dailyBackup(s *store.Store, dir string, retention int, now time.Time) error {
	name := "fleetglass-" + now.Format("2006-01-02") + ".db"
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := s.Backup(path); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "fleetglass-") || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}
		stamp := strings.TrimSuffix(strings.TrimPrefix(entry.Name(), "fleetglass-"), ".db")
		day, err := time.Parse("2006-01-02", stamp)
		if err == nil && day.Before(now.AddDate(0, 0, -retention)) {
			if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

func hostCommand(s *store.Store, args []string) {
	switch args[0] {
	case "list":
		if len(args) != 1 {
			log.Fatal("usage: host list")
		}
		h, err := s.HostRegistrations()
		if err != nil {
			log.Fatal(err)
		}
		if err := json.NewEncoder(os.Stdout).Encode(h); err != nil {
			log.Fatal(err)
		}
	case "add":
		if len(args) < 2 {
			log.Fatal("usage: host add <name> [--label k=v]")
		}
		name := args[1]
		labels := map[string]string{}
		for i := 2; i < len(args); i += 2 {
			if args[i] != "--label" || i+1 >= len(args) {
				log.Fatal("usage: host add <name> [--label k=v]")
			}
			parts := strings.SplitN(args[i+1], "=", 2)
			if len(parts) != 2 || !store.ValidHostName(parts[0]) || len(parts[1]) > 128 {
				log.Fatal("invalid label")
			}
			labels[parts[0]] = parts[1]
		}
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			log.Fatal(err)
		}
		token := base64.RawURLEncoding.EncodeToString(raw)
		if err := s.AddHost(name, labels, token); err != nil {
			log.Fatal(err)
		}
		fmt.Println(token)
	case "revoke", "remove":
		if len(args) < 2 || len(args) > 3 || !store.ValidHostName(args[1]) {
			log.Fatal("invalid host command")
		}
		var err error
		if args[0] == "revoke" {
			if len(args) != 2 {
				log.Fatal("usage: host revoke <name>")
			}
			err = s.RevokeHost(args[1])
		} else {
			if len(args) == 3 && args[2] != "--purge" {
				log.Fatal("usage: host remove <name> [--purge]")
			}
			err = s.RemoveHost(args[1], len(args) == 3)
		}
		if err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatal("unknown host command")
	}
}
