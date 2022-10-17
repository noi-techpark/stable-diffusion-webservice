package webservices

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"noi-sd-ws/utils"
	"strings"
)

/*
A GET request to `/getNextJob` gets the oldest job in queue that has state 'new'.

See README.md for details.
*/
func getNextJob(res http.ResponseWriter, req *http.Request) {

	res.Header().Set("content-type", "application/json")

	// parameters

	secret := strings.TrimSpace(req.URL.Query().Get("secret"))

	// validation

	errors := []string{}
	if secret == "" {
		errors = append(errors, "missing secret")
	}

	if len(errors) > 0 {
		res.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(res, "\"%s\"\n", strings.Join(errors, ", "))
		return
	}

	// open DB, check secret and get job

	db, err := sql.Open("sqlite3", DBName)
	defer db.Close()
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(res, "\"cannot open database\"\n")
		return
	}

	st, err := db.Prepare("select count(*) as cnt from secrets where kind = 'backend_secret' and secret = ?")
	defer st.Close()
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(res, "\"cannot query database\"\n")
		return
	}
	var cnt int64
	err = st.QueryRow(secret).Scan(&cnt)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(res, "\"cannot retrieve query results\"\n")
		return
	}
	if cnt < 1 {
		res.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(res, "\"wrong secret\"\n")
		return
	}

	st, err = db.Prepare("select token, number, width, height, prompt from jobs where state = 'new' order by created_ms limit 1")
	defer st.Close()
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(res, "\"cannot query database\"\n")
		return
	}
	type output_t struct {
		Token  string
		Number int64
		Width  int32
		Height int32
		Prompt string
	}
	var output output_t
	err = st.QueryRow().Scan(&output.Token, &output.Number, &output.Width, &output.Height, &output.Prompt)
	if err == sql.ErrNoRows {
		res.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(res, "\"no more jobs\"\n")
		return
	} else if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(res, "\"cannot retrieve query results\"\n")
		return
	}

	obj, _ := json.Marshal(output)

	// return job data

	res.WriteHeader(http.StatusOK)
	fmt.Fprintf(res, "%s\n", obj)
}

/*
A GET request to `/setJobStatus` updates the state of a
job (identified by its token).

See README.md for details.
*/
func setJobStatus(res http.ResponseWriter, req *http.Request) {

	res.Header().Set("content-type", "application/json")

	// parameters

	token := strings.TrimSpace(req.URL.Query().Get("token"))
	state := strings.TrimSpace(req.URL.Query().Get("state"))
	secret := strings.TrimSpace(req.URL.Query().Get("secret"))

	// validation

	errors := []string{}
	if token == "" {
		errors = append(errors, "missing token")
	} else if len(token) != 32 {
		errors = append(errors, "wrong size for token")
	}
	if secret == "" {
		errors = append(errors, "missing secret")
	}
	if state == "" {
		errors = append(errors, "missing state")
	} else if state != "pending" && state != "complete" {
		errors = append(errors, "invalid value for state")
	}

	if len(errors) > 0 {
		res.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(res, "\"%s\"\n", strings.Join(errors, ", "))
		return
	}

	// open DB, check secret and current state, apply new state

	db, err := sql.Open("sqlite3", DBName)
	defer db.Close()
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(res, "\"cannot open database\"\n")
		return
	}

	st, err := db.Prepare("select count(*) as cnt from secrets where kind = 'backend_secret' and secret = ?")
	defer st.Close()
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(res, "\"cannot query database\"\n")
		return
	}
	var cnt int64
	err = st.QueryRow(secret).Scan(&cnt)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(res, "\"cannot retrieve query results\"\n")
		return
	}
	if cnt < 1 {
		res.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(res, "\"wrong secret\"\n")
		return
	}

	st, err = db.Prepare("select state from jobs j where token = ?")
	defer st.Close()
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(res, "\"cannot query database\"\n")
		return
	}
	var old_state string
	err = st.QueryRow(token).Scan(&old_state)
	if err == sql.ErrNoRows {
		res.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(res, "\"token not found\"\n")
		return
	} else if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(res, "\"cannot retrieve query results\"\n")
		return
	}

	if !((old_state == "new" && state == "pending") || (old_state == "pending" && state == "complete")) {
		res.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(res, "\"invalid state transition\"\n")
		return
	}

	success := false
	tx, err := db.Begin()
	if err == nil {
		st, err := tx.Prepare("update jobs set state = ? where token = ?")
		defer st.Close()
		if err == nil {
			_, err = st.Exec(state, token)
			if err == nil {
				err = tx.Commit()
				if err == nil {
					success = true
				}
			}
		}
	}
	if !success {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(res, "\"cannot update job in database\"\n")
		return
	}

	utils.Log(fmt.Sprintf("setJobStatus: token=%s, new state = %s", token, state))

	// return ok

	res.WriteHeader(http.StatusOK)
	fmt.Fprintf(res, "\"ok\"\n")

}
