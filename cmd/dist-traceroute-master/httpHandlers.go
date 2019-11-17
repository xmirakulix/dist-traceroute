package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	valid "github.com/asaskevich/govalidator"
	"github.com/google/uuid"
	"github.com/xmirakulix/dist-traceroute/disttrace"
)

// status vars for webinterface
var lastTransmittedSlaveConfig = "none yet"
var lastTransmittedSlaveConfigTime time.Time

func checkSlaveCredentials(slave *disttrace.Slave, writer http.ResponseWriter, req *http.Request) bool {

	if disttrace.CheckSlaveAuth(db, slave.Name, slave.Password) {
		return true
	}

	// no match found, unauthorized!
	log.Warnf("checkCredentials: Unauthorized slave '%v', peer: %v", slave.Name, req.RemoteAddr)
	time.Sleep(2 * time.Second)
	http.Error(writer, "Unauthorized", http.StatusUnauthorized)
	return false
}

func httpHandleAPIAuth() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		user := req.URL.Query().Get("user")
		password := req.URL.Query().Get("password")

		log.Debugf("httpHandleAPIAuth: Received API 'auth' request for user<%v>", user)

		if user != "admin" || password != "123" {
			time.Sleep(3 * time.Second)
			http.Error(writer, "User/PW do not match", http.StatusUnauthorized)
			return
		}

		claims := disttrace.AuthClaims{
			Username: user,
		}
		token, err := disttrace.GetToken(claims)
		if err != nil {
			log.Warn("httpHandleAPIAuth: Can't generate auth token")
			http.Error(writer, "Can't generate token", http.StatusBadRequest)
			return
		}

		if _, err := writer.Write(token); err != nil {
			log.Warn("httpHandleAPIAuth: Couldn't write response: ", err)
		}

		log.Debug("httpHandleAPIAuth: Replying with success.")
		return
	}
}

func httpHandleAPIStatus() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		log.Debug("httpHandleAPIStatus: Received API 'status' request")

		var timeSinceSlaveCfg string
		if !lastTransmittedSlaveConfigTime.IsZero() {
			timeSinceSlaveCfg = time.Since(lastTransmittedSlaveConfigTime).Truncate(time.Second).String()
		}

		response := struct {
			Uptime              string
			LastSlaveConfigTime string
			LastSlaveConfig     string
		}{
			disttrace.GetUptime().Truncate(time.Second).String(),
			timeSinceSlaveCfg,
			lastTransmittedSlaveConfig,
		}

		resJSON, _ := json.MarshalIndent(response, "", "	")

		if _, err := writer.Write(resJSON); err != nil {
			log.Warn("httpHandleAPIStatus: Couldn't write response: ", err)
		}

		log.Debug("httpHandleAPIStatus: Replying with success.")
	}
}

func httpHandleAPIGraphData() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		dest := req.URL.Query().Get("dest")
		skip, _ := strconv.Atoi(req.URL.Query().Get("skip"))
		start := req.URL.Query().Get("start")
		end := req.URL.Query().Get("end")

		log.Debugf("httpHandleAPIGraphData: Received API 'graphdata' request, dest: <%v>, skip: <%v>, start<%v>, end<%v>", dest, skip, start, end)

		if len(dest) < 1 {
			log.Info("httpHandleAPIGraphData: Parameter dest missing or empty, returning error.")
			http.Error(writer, "Parameter dest missing or empty", http.StatusBadRequest)
			return
		}

		query := `
			SELECT MIN(dtStart) as start, MAX(dtStart) AS end, json_group_array(json_array(prevHopAddress, strHopIPAddress, cnt, avgDuration)) as links
			FROM (
				SELECT h.nHopIndex, t.dtStart, COALESCE(prev.strHopIPAddress, '0') as prevHopAddress,
				h.strHopDNSName,
				h.strHopIPAddress, COUNT(*) as cnt, AVG(h.dDurationSec)*1000 as avgDuration,
				h.strHopIPAddress || h.nHopIndex || COALESCE(prev.strHopIPAddress, '') AS LinkId,
				tg.strDestination

				FROM t_Hops h  
				JOIN t_Traceroutes t ON t.strTracerouteId = h.strTracerouteId
				JOIN t_Targets tg ON t.strTargetId = tg.strTargetId
				LEFT JOIN t_Hops prev ON h.strPreviousHopId = prev.strHopId

				WHERE tg.strDestination = ? AND h.nHopIndex > ?

				GROUP BY h.strHopIPAddress, h.nHopIndex, prevHopAddress
				ORDER BY h.nHopIndex
			) t
			GROUP BY t.strDestination 
			`

		resRow := db.QueryRow(query, dest, skip)
		var resStart, resEnd, resGraphData string
		if err := resRow.Scan(&resStart, &resEnd, &resGraphData); err != nil {
			if err == sql.ErrNoRows {
				log.Debug("httpHandleAPIGraphData: No data found, returning empty")
			} else {
				log.Warn("httpHandleAPIGraphData: Error while getting graph data, Error: ", err.Error())
			}
			resGraphData = "{}"
		}

		response := fmt.Sprintf("{ \"Start\": \"%v\", \"End\": \"%v\", \"Data\": %v }", resStart, resEnd, resGraphData)

		if _, err := io.WriteString(writer, response); err != nil {
			log.Warn("httpHandleAPIGraphData: Couldn't write response: ", err)
		}

		log.Debug("httpHandleAPIGraphData: Replying with success.")
	}
}

func httpHandleAPITraceHistory() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))

		log.Debugf("httpHandleAPITraceHistory: Received API 'tracehistory' request, limit: <%v>", limit)

		lastResultsQuery := `
		SELECT t.strTracerouteId, s.strSlaveName, tg.strDestination, strftime("%d.%m.%Y %H:%M", t.dtStart) AS dtStart, COUNT(h.strHopId) AS nHopCount, 
			json_group_object(h.nHopIndex, json_object('IP', h.strHopIPAddress, 'DNS', h.strHopDNSName, 'Duration', h.dDurationSec)) AS strHopDetails
		FROM t_Traceroutes t 
		JOIN t_Slaves s ON t.strSlaveId = s.strSlaveId 
		JOIN t_Targets tg ON t.strTargetId = tg.strTargetId
		LEFT JOIN t_Hops h ON t.strTracerouteId = h.strTracerouteId
		GROUP BY t.strTracerouteId
		ORDER BY t.dtStart DESC 
		`

		if limit != 0 {
			lastResultsQuery += "LIMIT " + strconv.Itoa(limit)
		}

		var resRows *sql.Rows
		var err error

		if resRows, err = db.Query(lastResultsQuery); err != nil {
			log.Warn("httpHandleAPITraceHistory: Couldn't get last results from DB, Error: ", err)
			http.Error(writer, "Couldn't get last results from DB", http.StatusInternalServerError)
			return
		}
		defer resRows.Close()

		type trace struct {
			TraceID    uuid.UUID
			HopCnt     int64
			SlaveName  string
			DestName   string
			DetailJSON string
			StartTime  string
		}

		rows := []trace{}

		for resRows.Next() {
			var t trace
			if err = resRows.Scan(&t.TraceID, &t.SlaveName, &t.DestName, &t.StartTime, &t.HopCnt, &t.DetailJSON); err != nil {
				log.Warn("httpHandleAPITraceHistory: Couldn't read DB result set, Error: ", err)
				http.Error(writer, "Couldn't read DB result set", http.StatusInternalServerError)
				return
			}
			rows = append(rows, t)
		}

		var response []byte
		if response, err = json.MarshalIndent(rows, "", "	"); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}

		if _, err := writer.Write(response); err != nil {
			log.Warn("httpHandleAPITraceHistory: Couldn't write response: ", err)
		}

		log.Debug("httpHandleAPITraceHistory: Replying with success.")
	}
}

func httpDefaultHandler() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		log.Info("httpDefaultHandler: Received request for base/unknown URL, returning 'Not Found': ", req.URL)

		// reply with error
		http.Error(writer, "Not found", http.StatusNotFound)
		return
	}
}

func httpHandleSlaveResults() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		log.Debug("httpHandleSlaveResults: Received request results, URL: ", req.URL)

		// init vars
		result := disttrace.TraceResult{}
		jsonDecoder := json.NewDecoder(req.Body)

		// decode request
		err := jsonDecoder.Decode(&result)
		if err != nil {
			log.Warn("httpHandleSlaveResults: Couldn't decode request body into JSON: ", err)

			// create error response
			response := disttrace.SubmitResult{
				Success:       false,
				Error:         "Couldn't decode request body into JSON: " + err.Error(),
				RetryPossible: false,
			}

			var responseJSON []byte
			if responseJSON, err = json.Marshal(response); err != nil {
				http.Error(writer, "Error: Couldn't marshal error response into JSON", http.StatusBadRequest)
				log.Warn("httpHandleSlaveResults: Error: Couldn't marshal error response into JSON: ", err)
				return
			}

			// reply with error
			http.Error(writer, string(responseJSON), http.StatusBadRequest)
			return
		}

		// check authorization
		if !checkSlaveCredentials(&result.Slave, writer, req) {
			return
		}

		log.Infof("httpHandleSlaveResults: Received results from slave '%v' for target '%v'. Success: %v, Hops: %v.",
			result.Slave.Name, result.Target.Name,
			result.Success, result.HopCount,
		)

		if ok, e := disttrace.ValidateTraceResult(result); !ok || e != nil {
			log.Warn("httpHandleSlaveResults: Result validation failed, Error: ", e)
			http.Error(writer, "Result validation failed: "+e.Error(), http.StatusBadRequest)
			return
		}

		// store submitted result
		var tx *disttrace.Tx
		var errDb error
		errDb = nil

		if tx, errDb = db.Begin(); err != nil {
			log.Warn("httpHandleSlaveResults: Error creating database transaction while storing result, Error: ", errDb)
			http.Error(writer, "Database error", http.StatusInternalServerError)
			return
		}
		// catch errors and rollback!
		defer func() {
			if errDb != nil {
				log.Warn("httpHandleSlaveResults: Caught error during database operations, rolling transaction back!")
				tx.Rollback()
			}
		}()

		// prepare traceroute insert
		traceStmt, errDb := tx.Prepare(`
			INSERT INTO t_Traceroutes (strTracerouteId, strSlaveId, strTargetId, dtStart, strAnnotations) 
			VALUES (?, ?, ?, ?, ?) 
			`)
		defer traceStmt.Close()
		if errDb != nil {
			log.Warn("httpHandleSlaveResults: Error while preparing database statement, Error: ", errDb)
			http.Error(writer, "Database error", http.StatusInternalServerError)
			return
		}

		// prepare hop insert
		hopStmt, errDb := tx.Prepare(`
			INSERT INTO t_Hops (strHopId, strTracerouteId, nHopIndex, strHopIPAddress, strHopDNSName, dDurationSec, strPreviousHopId)	
			VALUES (?, ?, ?, ?, ?, ?, ?) 
			`)
		defer hopStmt.Close()
		if errDb != nil {
			log.Warn("httpHandleSlaveResults: Error while preparing database statement, Error: ", errDb)
			http.Error(writer, "Database error", http.StatusInternalServerError)
			return
		}

		log.Debug("httpHandleSlaveResults: Finished preparing queries, inserting data...")

		// Insert result info
		traceID := uuid.New()
		if _, errDb := traceStmt.Exec(traceID, result.Slave.ID, result.Target.ID, result.DateTime.Format(time.RFC3339), ""); errDb != nil {
			log.Warn("httpHandleSlaveResults: Error while inserting result, Error: ", errDb)
			http.Error(writer, "Database error", http.StatusInternalServerError)
			return
		}

		if errDb != nil {
			log.Warn("httpHandleSlaveResults: Error while getting last inserted ID of traceroute, Error: ", errDb)
			http.Error(writer, "Database error", http.StatusInternalServerError)
			return
		}
		log.Debug("httpHandleSlaveResults: Inserted result with ID: ", traceID)

		// Insert hops info
		var prevHopID uuid.UUID
		for _, hop := range result.Hops {

			hopID := uuid.New()
			// prev hop is null on first hop
			if hop.TTL == 0 {
				_, errDb = hopStmt.Exec(hopID, traceID, hop.TTL, hop.AddressString(), hop.Host, hop.ElapsedTime.Seconds(), nil)
			} else {
				_, errDb = hopStmt.Exec(hopID, traceID, hop.TTL, hop.AddressString(), hop.Host, hop.ElapsedTime.Seconds(), prevHopID)
			}
			if errDb != nil {
				log.Warn("httpHandleSlaveResults: Error while inserting hop, Error: ", errDb)
				http.Error(writer, "Database error", http.StatusInternalServerError)
				return
			}
			prevHopID = hopID
			if errDb != nil {
				log.Warn("httpHandleSlaveResults: Error while getting last inserted ID of hop, Error: ", errDb)
				http.Error(writer, "Database error", http.StatusInternalServerError)
				return
			}
		}
		log.Debug("httpHandleSlaveResults: Successfully inserted trace info and hops, commiting transaction...")

		if errDb = tx.Commit(); errDb != nil {
			log.Warn("httpHandleSlaveResults: Error while commiting transaction, Error: ", errDb)
			http.Error(writer, "Database error", http.StatusInternalServerError)
			return
		}

		// reply with success
		response := disttrace.SubmitResult{
			Success:       true,
			Error:         "",
			RetryPossible: true,
		}

		responseJSON, err := json.Marshal(response)
		if err != nil {
			http.Error(writer, "Error: Couldn't marshal success response into JSON", http.StatusInternalServerError)
			log.Warn("httpHandleSlaveResults: Error: Couldn't marshal success response into JSON: ", err)
			return
		}

		// Success!
		_, err = io.WriteString(writer, string(responseJSON))
		if err != nil {
			log.Warn("httpHandleSlaveResults: Couldn't write success response: ", err)
		}
		log.Debug("httpHandleSlaveResults: Replying success.")
		return
	}
}

func httpHandleSlaveConfig() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		log.Debug("httpHandleSlaveConfig: Received request for config, URL: ", req.URL)

		var err error

		// read request body
		var reqBody []byte
		if reqBody, err = ioutil.ReadAll(req.Body); err != nil {
			log.Warn("httpHandleSlaveConfig: Can't read request body, Error: ", err)
			http.Error(writer, "Can't read request", http.StatusInternalServerError)
			return
		}

		// parse JSON from request body
		var slave disttrace.Slave
		if err = json.Unmarshal(reqBody, &slave); err != nil {
			log.Warn("httpHandleSlaveConfig: Can't unmarshal request body into slave, Error: ", err)
			http.Error(writer, "Can't unmarshal request body", http.StatusBadRequest)
			return
		}

		// check authorization
		if !checkSlaveCredentials(&slave, writer, req) {
			return
		}

		// read config from db
		slaveConf := disttrace.SlaveConfig{}

		query := "SELECT strTargetId, strDescription, strDestination, nRetries, nMaxHops, nTimeoutMSec FROM t_Targets"
		rows, err := db.Query(query)
		if err != nil {
			http.Error(writer, "Error: Can't read targets from db", http.StatusInternalServerError)
			log.Warn("httpHandleSlaveConfig: Can't read targets from db, Error: ", err)
			lastTransmittedSlaveConfig = "Error: Can't read targets from db: " + err.Error()
			lastTransmittedSlaveConfigTime = time.Now()
			return
		}
		defer rows.Close()

		for rows.Next() {
			var tgt = disttrace.TraceTarget{}

			if err := rows.Scan(&tgt.ID, &tgt.Name, &tgt.Address, &tgt.Retries, &tgt.MaxHops, &tgt.TimeoutMs); err != nil {
				http.Error(writer, "Error: Can't scan target rows", http.StatusInternalServerError)
				log.Warn("httpHandleSlaveConfig: Can't scan target rows, Error: ", err)
				lastTransmittedSlaveConfig = "Error: Can't scan target rows: " + err.Error()
				lastTransmittedSlaveConfigTime = time.Now()
				return
			}
			slaveConf.Targets = append(slaveConf.Targets, tgt)
		}

		// validate config
		if ok, e := valid.ValidateStruct(slaveConf); !ok || e != nil {
			http.Error(writer, "Error: Loaded config is invalid", http.StatusInternalServerError)
			log.Warn("httpHandleSlaveConfig: Loaded config is invalid, Error: ", e)
			lastTransmittedSlaveConfig = "Error: Loaded config is invalid: " + err.Error()
			lastTransmittedSlaveConfigTime = time.Now()
			return
		}

		body, err := json.MarshalIndent(slaveConf, "", "	")
		if err != nil {
			http.Error(writer, "Error: Couldn't marshal slaves for response", http.StatusInternalServerError)
			log.Warn("httpHandleSlaveConfig: Couldn't marshal slaves for response, Error: ", err)
			lastTransmittedSlaveConfig = "Error: Couldn't marshal slaves for response: " + err.Error()
			lastTransmittedSlaveConfigTime = time.Now()
			return
		}

		// send config to slave
		lastTransmittedSlaveConfig = string(body)
		lastTransmittedSlaveConfigTime = time.Now()
		_, err = io.WriteString(writer, string(body))
		if err != nil {
			log.Warn("httpHandleSlaveConfig: Couldn't write success response: ", err)
			return
		}

		log.Infof("httpHandleSlaveConfig: Transmitting currently configured targets to slave '%v' for %v targets", slave.Name, len(slaveConf.Targets))
		return
	}
}

func checkJWTAuth(writer http.ResponseWriter, req *http.Request, next http.HandlerFunc) {

	authHeader := req.Header.Get("Authorization")
	log.Debugf("checkAuth: Received request, checking Auth-Header<%v>", authHeader)

	if authHeader == "" {
		log.Debug("checkAuth: Auth header empty or missing, returning unauthorized...")
		http.Error(writer, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// check for a verifiable token
	if err := disttrace.VerifyToken([]byte(disttrace.TokenFromAuthHeader(authHeader))); err != nil {
		log.Debug("checkAuth: Couldn't verify supplied token, returning unauthorized...")
		http.Error(writer, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// call next handler in chain
	next(writer, req)
}

func handleAccessControl(writer http.ResponseWriter, req *http.Request, next http.HandlerFunc) {

	writer.Header().Add("Access-Control-Allow-Origin", "*")
	writer.Header().Add("Access-Control-Allow-Methods", "DELETE, POST, GET, OPTIONS")
	writer.Header().Add("Access-Control-Allow-Headers", "Content-Type, Access-Control-Allow-Headers, Authorization, X-Requested-With")

	if req.Method == "OPTIONS" {
		writer.WriteHeader(http.StatusOK)
		return
	}

	// call next handler in chain
	next(writer, req)
}

func httpHandleAPIUsers() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		log.Debug("httpHandleAPIUsers: Received API 'users' request")
	}
}

func httpHandleAPISlaves() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		log.Debug("httpHandleAPISlaves: Received API 'slaves' request")

		type Slave struct {
		}

	}
}

func httpHandleAPITargets() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		log.Debug("httpHandleAPITargets: Received API 'targets' request")

	}
}
