// SPDX-FileCopyrightText: NOI Techpark <digital@noi.bz.it>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package webservices

import (
	"fmt"
	"log"
	"net/http"
	"noi-sd-ws/utils"
)

var Secrets = map[string]string{}
var DBName string

/*
spawn web services
*/
func Spawn() {

	const ADDRESS = "0.0.0.0:9090"

	mux := http.NewServeMux()

	// frontend API

	mux.Handle("/addJob", http.HandlerFunc(addJob))
	mux.Handle("/getJobStatus", http.HandlerFunc(getJobStatus))

	// backend API

	mux.Handle("/getNextJob", http.HandlerFunc(getNextJob))
	mux.Handle("/setJobStatus", http.HandlerFunc(setJobStatus))

	utils.Log(fmt.Sprintf("service listening at %s", ADDRESS))
	err := http.ListenAndServe(ADDRESS, mux)
	if err != nil {
		log.Fatal(err)
	}
}
