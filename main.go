package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ErikJermanis/evcal/store"
)

const usage = `Usage:
  evcal                 start the server (same as "evcal serve")
  evcal pair <name>     print a one-use pairing link for a device, valid 24 h
  evcal devices         list paired devices
  evcal revoke <id>     sign a device out
  evcal backup <file>   write a consistent copy of the database to <file>

Settings come from the environment or a .env file in the working directory:
  PORT (8081), DATABASE_PATH (data/evcal.sqlite), PUBLIC_BASE_URL (http://localhost:PORT)`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cmd := "serve"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	if cmd == "help" || cmd == "-h" || cmd == "--help" {
		fmt.Println(usage)
		return nil
	}

	cfg := loadConfig()
	st, err := store.Open(cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer st.Close()

	switch cmd {
	case "serve":
		return serve(cfg, st)
	case "pair":
		name := strings.TrimSpace(strings.Join(args, " "))
		if name == "" {
			return errors.New(`a device name is required, e.g. evcal pair "Erik - iPhone"`)
		}
		token, err := st.CreatePairingToken(name)
		if err != nil {
			return err
		}
		fmt.Printf("%s/pair/%s\n", cfg.PublicBaseURL, token)
		return nil
	case "devices":
		devices, err := st.ListDevices()
		if err != nil {
			return err
		}
		if len(devices) == 0 {
			fmt.Println("No paired devices.")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tSTATUS\tPAIRED\tLAST SEEN")
		for _, d := range devices {
			status := "active"
			if d.RevokedAt != "" {
				status = "revoked " + d.RevokedAt
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", d.ID, d.Name, status, d.CreatedAt, d.LastSeenAt)
		}
		return w.Flush()
	case "revoke":
		if len(args) != 1 {
			return errors.New("usage: evcal revoke <id> (see evcal devices)")
		}
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid device id %q", args[0])
		}
		if err := st.RevokeDevice(id); errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("no active device with id %d", id)
		} else if err != nil {
			return err
		}
		fmt.Printf("Revoked device %d.\n", id)
		return nil
	case "backup":
		if len(args) != 1 {
			return errors.New("usage: evcal backup <file>")
		}
		if err := st.Backup(args[0]); err != nil {
			return err
		}
		fmt.Println("Backed up to", args[0])
		return nil
	}
	return fmt.Errorf("unknown command %q\n\n%s", cmd, usage)
}

func serve(cfg config, st *store.Store) error {
	handler, err := newServer(cfg, st)
	if err != nil {
		return err
	}
	addr := "127.0.0.1:" + cfg.Port
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("Listening on http://%s (database %s)", addr, cfg.DatabasePath)
	return srv.ListenAndServe()
}
