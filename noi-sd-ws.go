// SPDX-FileCopyrightText: NOI Techpark <digital@noi.bz.it>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"noi-sd-ws/utils"
	"noi-sd-ws/webservices"
	"os"
	"path/filepath"
)

func main() {

	utils.Log("NOI SD webservice 1.00")

	// open SQLite DB, load secrets and print jobs table stats

	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <SQLlite.db>", filepath.Base(os.Args[0]))
		os.Exit(1)
	}

	webservices.DBName = os.Args[1]

	db, err := sql.Open("sqlite3", webservices.DBName)
	if err != nil {
		log.Fatal(err)
	}

	rows, err := db.Query("select kind, secret from secrets")
	if err != nil {
		log.Fatal(err)
	}
	for rows.Next() {
		var kind, secret string
		err = rows.Scan(&kind, &secret)
		if err == nil {
			webservices.Secrets[kind] = secret
		}
	}

	utils.Log(fmt.Sprintf("database: %d secrets read", len(webservices.Secrets)))

	rows, err = db.Query("select state, count(*) as c from jobs group by state order by state")
	if err != nil {
		log.Fatal(err)
	}
	for rows.Next() {
		var state string
		var c int64
		err = rows.Scan(&state, &c)
		if err == nil {
			utils.Log(fmt.Sprintf("database: %d jobs in state '%s'", c, state))
		}
	}

	db.Close()

	// spawn web services

	webservices.Spawn()

}
