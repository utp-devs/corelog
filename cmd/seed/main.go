// Command seed fills a CoreLog database with deterministic, scenario-based data.
//
// Usage:
//
//	SEED_PASSWORD=... go run ./cmd/seed --scenario demo
//
// Every seeded account uses the password from SEED_PASSWORD, which is required,
// has no default and is never printed. Scenarios register themselves in init
// functions (see scenario_*.go), so new scenarios do not touch this file.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jhonsferg/corelog/internal/platform/config"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

const (
	envSeedPassword = "SEED_PASSWORD"
	defaultIssues   = 50000
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("seed", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	scenario := fs.String("scenario", "demo", "escenario a sembrar: "+strings.Join(scenarioNames(), ", "))
	issues := fs.Int("issues", defaultIssues, "numero de incidencias para --scenario scale")
	forceReset := fs.Bool("force-reset", false, "vacia las tablas de la aplicacion antes de sembrar")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("argumento no reconocido: %s", fs.Arg(0))
	}

	password := os.Getenv(envSeedPassword)
	if password == "" {
		return fmt.Errorf("%s es obligatorio: define la contrasena de las cuentas de prueba (no tiene valor por defecto)", envSeedPassword)
	}
	sc, err := lookupScenario(*scenario)
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("cargar configuracion: %w", err)
	}
	db, err := database.NewPostgres(cfg.Database)
	if err != nil {
		return fmt.Errorf("conectar a la base de datos: %w", err)
	}
	defer func() { _ = db.Close() }()

	return Execute(context.Background(), db, sc, Options{
		Password:   password,
		Issues:     *issues,
		ForceReset: *forceReset,
		Out:        out,
	})
}
