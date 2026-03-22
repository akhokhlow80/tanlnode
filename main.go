package main

import (
	"akhokhlow80/tanlnode/db"
	"akhokhlow80/tanlnode/sqlgen"
	"akhokhlow80/tanlnode/subnets"
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"strings"

	_ "akhokhlow80/tanlnode/docs"

	httpSwagger "github.com/swaggo/http-swagger/v2"

	"github.com/caarlos0/env/v11"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

type config struct {
	HTTPBind              string `env:"HTTP_BIND,required"`
	DBPath                string `env:"DB_PATH,required"`
	WGInterface           string `env:"WG_IF,required"`
	WGAllocNets           string `env:"WG_ALLOC_NETS,required"`
	WGDNS                 string `env:"WG_DNS"`
	WGMTU                 int    `env:"WG_MTU"`
	WGPublicKey           string `env:"WG_PUBLIC_KEY,required"`
	WGEndpoint            string `env:"WG_ENDPOINT,required"`
	WGPersistentKeepalive int    `env:"WG_PERSISTENT_KEEPALIVE"`
}

type node struct {
	cfg *config
	// db mutex is used so synchronize both db and nettree operations
	db      db.DB
	subnets subnets.Service
}

//go:embed sql/migrations/*.sql
var embedMigrations embed.FS

func (node *node) initDB(dbPath string) error {
	var err error
	node.db.DB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}

	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("sqlite"); err != nil {
		return err
	}
	if err := goose.Up(node.db.DB, "sql/migrations"); err != nil {
		return err
	}

	node.db.Queries = sqlgen.New(node.db.DB)

	return nil
}

func parseAllocNets(str string) ([]netip.Prefix, error) {
	var nets []netip.Prefix
	for netStr := range strings.SplitSeq(str, ",") {
		netStr = strings.Trim(netStr, " \t")
		net, err := netip.ParsePrefix(netStr)
		if err != nil {
			return nil, err
		}
		if net.Masked().Addr() != net.Addr() {
			return nil, fmt.Errorf("Alloc net %s has non-zero host bits", net)
		}
		nets = append(nets, net)
	}
	// TODO: optimize?
	for i := range nets {
		for j := i + 1; j < len(nets); j++ {
			if nets[i].Contains(nets[j].Addr()) || nets[j].Contains(nets[i].Addr()) {
				return nil, fmt.Errorf("Networks %s and %s overlap", nets[i], nets[j])
			}
		}
	}
	return nets, nil
}

func (node *node) listen() {
	apiV1 := http.NewServeMux()
	node.registerSubnetHandlers(apiV1)
	node.registerPeerHandlers(apiV1)

	root := http.NewServeMux()
	root.Handle("/api/v1/", http.StripPrefix("/api/v1", apiV1))

	root.Handle("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))
	log.Printf("Binding to %s", node.cfg.HTTPBind)
	log.Fatal(http.ListenAndServe(node.cfg.HTTPBind, root))
}

func main() {
	cfg, err := env.ParseAs[config]()
	if err != nil {
		log.Fatalf("Failed to parse env: %s", err)
	}

	var node node
	node.cfg = &cfg

	err = node.initDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to init db: %s", err)
	}

	allocNets, err := parseAllocNets(cfg.WGAllocNets)
	if err != nil {
		log.Fatalf("Failed to parse alloc nets: %s", err)
	}
	node.subnets, err = subnets.NewService(context.Background(), allocNets, &node.db)
	if err != nil {
		log.Fatalf("Failed to init subnets service: %s", err)
	}

	node.listen()
}
