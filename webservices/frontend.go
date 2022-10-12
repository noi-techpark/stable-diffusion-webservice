package webservices

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"noi-sd-ws/utils"
	"strconv"
	"strings"
	"time"
)

/*
A GET request to /addJob adds as new job to the queue.

See README.md for details.
*/
func addJob(res http.ResponseWriter, req *http.Request) {

	res.Header().Set("content-type", "application/json")
	res.Header().Set("Access-Control-Allow-Origin", "*")

	// hard coded limits (if you change these, don't forget to update README.md)

	const MAX_PROMPT_BYTES = 500
	const MAX_NUMBER = 16
	VALID_RESOLUTIONS := map[string]int{
		"1024x576": 1, // (wide landscape 16:9)
		"768x576":  1, // (landscape 4:3)
		"576x768":  1, // (portrait 3:4),
		"576x576":  1, // (square 1:1),
	}
	const MAX_CAPTCHA_BYTES = 5000

	// parameters

	prompt := strings.TrimSpace(req.URL.Query().Get("prompt"))
	number_str := strings.TrimSpace(req.URL.Query().Get("number"))
	resolution := strings.TrimSpace(req.URL.Query().Get("resolution"))
	captcha := strings.TrimSpace(req.URL.Query().Get("captcha"))

	// validation

	errors := []string{}

	if prompt == "" {
		errors = append(errors, "missing prompt")
	} else if len(prompt) > MAX_PROMPT_BYTES {
		errors = append(errors, "prompt too long")
	}
	lutsrc := []string{"ä", "ö", "ü", "Ä", "Ö", "Ü", "ß",
		"à", "è", "ì", "ò", "ù", "á", "é", "í", "ó", "ú",
		"À", "È", "Ì", "Ò", "Ù", "Á", "É", "Í", "Ó", "Ú"}
	lutdst := []string{"ae", "oe", "ue", "Ae", "Oe", "Ue", "ss",
		"a", "e", "i", "o", "u", "a", "e", "i", "o", "u",
		"A", "E", "I", "O", "U", "A", "E", "I", "O", "U"}
	for i := range lutsrc {
		prompt = strings.Replace(prompt, lutsrc[i], lutdst[i], -1)
	}
	for _, p := range prompt {
		if int(p) < 32 || int(p) > 126 {
			errors = append(errors, "prompt contains a non-printable ASCII character (after transliteration of DE/IT)")
		}
	}

	number, parseErr := strconv.ParseInt(number_str, 10, 64)
	if number_str == "" {
		errors = append(errors, "missing number")
	} else if parseErr != nil {
		errors = append(errors, "cannot parse number")
	} else if number < 1 || number > MAX_NUMBER {
		errors = append(errors, "number out of range")
	}

	if resolution == "" {
		errors = append(errors, "missing resolution")
	} else if VALID_RESOLUTIONS[resolution] != 1 {
		errors = append(errors, "invalid resolution")
	}
	resArr := strings.Split(resolution, "x")
	width, _ := strconv.ParseInt(resArr[0], 10, 32)
	height, _ := strconv.ParseInt(resArr[1], 10, 32)

	if captcha == "" {
		errors = append(errors, "missing captcha")
	} else if len(captcha) > MAX_CAPTCHA_BYTES {
		errors = append(errors, "captcha too long")
	}

	// verify captcha

	if !PostHCaptcha(captcha) {
		errors = append(errors, "captcha failed")
	}

	if len(errors) > 0 {
		res.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(res, "\"%s\"\n", strings.Join(errors, ", "))
		return
	}

	// generate random token

	token, tokenErr := utils.GetRandomToken()

	if tokenErr != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(res, "\"crypto RNG failed\"\n")
		return
	}

	// write new job to SQLite DB

	db, err := sql.Open("sqlite3", DBName)
	defer db.Close()
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(res, "\"cannot open database\"\n")
		return
	}
	success := false
	tx, err := db.Begin()
	if err == nil {
		st, err := tx.Prepare("insert into jobs(created_ms, prompt, number, width, height, token, state, completed_ms) values(?, ?, ?, ?, ?, ?, ?, ?)")
		defer st.Close()
		if err == nil {
			_, err = st.Exec(time.Now().UnixMilli(), prompt, number, width, height, token, "new", 0)
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
		fmt.Fprintf(res, "\"cannot write job to database\"\n")
		return
	}

	utils.Log(fmt.Sprintf("addJob: num = %2d, res = %4dx%4d, token=%s, prompt = %s", number, width, height, token, prompt))

	// return random token

	res.WriteHeader(http.StatusOK)
	fmt.Fprintf(res, "\"%s\"\n", token)
}

/*
A GET request to `/getJobStatus` returns information about a
job (identified by its token).

See README.md for details.
*/
func getJobStatus(res http.ResponseWriter, req *http.Request) {

	res.Header().Set("content-type", "application/json")
	res.Header().Set("Access-Control-Allow-Origin", "*")

	// parameters

	token := strings.TrimSpace(req.URL.Query().Get("token"))

	// validation

	errors := []string{}
	if token == "" {
		errors = append(errors, "missing token")
	} else if len(token) != 32 {
		errors = append(errors, "wrong size for token")
	}

	if len(errors) > 0 {
		res.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(res, "\"%s\"\n", strings.Join(errors, ", "))
		return
	}

	// get job data from db

	db, err := sql.Open("sqlite3", DBName)
	defer db.Close()
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(res, "\"cannot open database\"\n")
		return
	}

	st, err := db.Prepare("select token, state, created_ms, " +
		"(select count(*) from jobs j2 where j2.created_ms < j1.created_ms and j2.state != 'complete') as qlen " +
		"from jobs j1 where token = ?")
	defer st.Close()
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(res, "\"cannot query database\"\n")
		return
	}
	type output_t struct {
		Token       string
		State       string
		Age         float64
		QueueLength int64
	}
	var output output_t
	var created_ms int64
	err = st.QueryRow(token).Scan(&output.Token, &output.State, &created_ms, &output.QueueLength)
	if err == sql.ErrNoRows {
		res.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(res, "\"token not found\"\n")
		return
	} else if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(res, "\"cannot retrieve query results\"\n")
		return
	}
	output.Age = 0.001 * (float64(time.Now().UnixMilli()) - float64(created_ms))
	obj, _ := json.Marshal(output)

	// return job data

	res.WriteHeader(http.StatusOK)
	fmt.Fprintf(res, "%s\n", obj)

}

func PostHCaptcha(captcha string) bool {

	// POST to hcaptcha and read response
	// if something fails, return false

	utils.Log("POSTing to hcaptcha.com")
	resp, err := http.PostForm("https://hcaptcha.com/siteverify", url.Values{"response": {captcha}, "secret": {Secrets["hcaptcha_secret"]}})
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	// extract key "success" from resulting JSON and return it,
	// again, if something fails, return false

	type response_t struct {
		Success bool
	}
	var response response_t
	response.Success = false

	err = json.Unmarshal(body, &response)
	if err != nil {
		return false
	}
	utils.Log(fmt.Sprintf("hcaptcha.com answer verbatim: %s", body))
	utils.Log(fmt.Sprintf("hcaptcha.com answer contains \"success=true\": %t", response.Success))

	return response.Success
}
