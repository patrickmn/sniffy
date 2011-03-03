package main

import (
	"github.com/pmylund/sniffy/common"

	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type StateFunc func(string)

const (
	stateLimbo = iota
	stateLogin
	stateMenu
)

var (
	cliState      int
	stateHandlers = map[int]StateFunc{
		stateLogin: login,
		stateMenu:  menu,
	}
)

func showMenu() {
	fmt.Println(`
Choices
-----
D) Change database connection settings
H) Change web server host/port
Q) Log out`)
}

func prompt(state int) {
	in := bufio.NewReader(os.Stdin)
	cliState = state
	for {
		if cliState == stateLimbo {
			return
		}
		line, err := in.ReadString('\n')
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		str := strings.TrimSpace(line)
		stateHandlers[cliState](str)
	}
}

func login(s string) {
	password := "woosha"
	if password != "" && s == password {
		showMenu()
		cliState = stateMenu
		return
	}
	<-time.After(3 * time.Second)
	fmt.Println("Invalid password, or the administrator password was not set")
	fmt.Print("Password: ")
}

func menu(s string) {
	c := strings.ToLower(s)
	switch c {
	default:
		fmt.Println("Invalid choice")
		showMenu()
	case "d":
		cliState = stateLimbo
	case "h":
	case "q":
		fmt.Println("Password:")
		cliState = stateLogin
	}
}

func setup() {
	in := bufio.NewReader(os.Stdin)
	err := setupDatabase(in)
	if err != nil {
		fmt.Println("Setup failed:", err)
		os.Exit(1)
	}
	fmt.Println("")
	fmt.Println("Configuration completed.")
	fmt.Println("")
	boot()
}

func checkError(err error) {
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func getVal(in *bufio.Reader) string {
	line, err := in.ReadString('\n')
	checkError(err) // It's okay for this func to be exiting the program
	return strings.TrimSpace(line)
}

func setupDatabase(in *bufio.Reader) error {
	var (
		c = &SniffyConfig{
			// Defaults
			DBType: "postgres", // TODO: Type selection later
			DBHost: "localhost",
			DBPort: 5432,
			DBName: "sniffy",
			DBUser: "sniffy",
		}
		val string
	)
	fmt.Println(`Sniffy needs a PostgreSQL database backend to store its data.

  - If PostgreSQL is not installed, please hit Ctrl+C, install it with e.g.
    'sudo apt-get install postgresql', then re-run Sniffy.

  - If a database does not already exist, enter the desired settings,
    then 'sql' to generate an SQL statement which you can run to create
    the user and database with optimal settings. (Leave password blank to
    create a random password.)
`)
	fmt.Printf("Host  [%s:%d]: ", c.DBHost, c.DBPort)
	val = getVal(in)
	if val != "" {
		split := strings.Split(val, ":")
		c.DBHost = split[0]
		l := len(split)
		switch l {
		default:
			fmt.Printf("Invalid host designation; using %s:%d\n", c.DBHost, c.DBPort)
		case 1:
		case 2:
			port, err := strconv.Atoi(split[1])
			if err != nil {
				fmt.Printf("Invalid port number; using %d\n", c.DBPort)
			} else {
				c.DBPort = port
			}
		}
	}

	fmt.Printf("DB name       [%s]: ", c.DBName)
	val = getVal(in)
	if val != "" {
		c.DBName = val
	}

	fmt.Printf("Username      [%s]: ", c.DBUser)
	val = getVal(in)
	if val != "" {
		c.DBUser = val
	}

	fmt.Print("Password              : ")
	val = getVal(in)
	c.DBPass = val

	config = c // Assign to global variable

	checkError(setupDatabaseTryConnect(in))
	checkError(saveConfig())
	return nil
}

func setupDatabaseTryConnect(in *bufio.Reader) error {
	checkError(connectDB())
	_, err := db.Exec("SELECT datname FROM pg_database WHERE datname = $1 LIMIT 1", config.DBName)
	if err != nil {
		fmt.Printf("\nThere was an error connecting to the database: %v\n\n", err)
		fmt.Println("To see the SQL needed to create the database and user, type 'sql'.\n")
		fmt.Print("Reconfigure the database settings [yes]? ")
		line, oerr := in.ReadString('\n')
		checkError(oerr)
		ans := strings.TrimSpace(line)
		fmt.Println("")
		if strings.HasPrefix("yes", ans) {
			setupDatabase(in)
		} else if ans == "sql" {
			if config.DBPass == "" {
				config.DBPass = common.RandomString(32)
			}

			sql := `CREATE USER ` + common.EscapeSQL(config.DBUser) + `;
ALTER USER ` + common.EscapeSQL(config.DBUser) + ` WITH PASSWORD '` + common.EscapeSQL(config.DBPass) + `';
CREATE DATABASE ` + common.EscapeSQL(config.DBName) + `
  WITH OWNER      = ` + common.EscapeSQL(config.DBUser) + `
       ENCODING   = 'UTF8'
       TABLESPACE = pg_default
       LC_COLLATE = 'en_US.UTF-8'
       LC_CTYPE   = 'en_US.UTF-8'
       CONNECTION LIMIT = -1;
GRANT ALL PRIVILEGES ON DATABASE ` + common.EscapeSQL(config.DBName) + ` TO ` + common.EscapeSQL(config.DBUser) + `;`
			fmt.Println("-- SQL statement")
			fmt.Println("--------------------------------------")
			fmt.Println(sql)
			fmt.Println("--------------------------------------")
			fmt.Println("")
			fmt.Println("Run 'sudo -u postgres psql postgres' and paste the above SQL to create the user and database.")
			fmt.Println("")
			fmt.Print("Try to connect again [yes]? ")
			line, oerr := in.ReadString('\n')
			checkError(oerr)
			ans := strings.TrimSpace(line)
			if strings.HasPrefix("yes", ans) {
				setupDatabaseTryConnect(in)
				return nil
			}
		}
		return err
	}
	fmt.Println("")
	fmt.Println("Database connection established.")
	return nil
}
