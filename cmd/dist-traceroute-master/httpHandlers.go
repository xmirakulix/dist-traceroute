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
	"github.com/gorilla/mux"
	"github.com/xmirakulix/dist-traceroute/disttrace"
)

// status vars for webinterface
var lastTransmittedSlaveConfig = "none yet"
var lastTransmittedSlaveConfigTime time.Time

func checkSlaveCredentials(slave *disttrace.Slave, writer http.ResponseWriter, req *http.Request) (bool, uuid.UUID) {

	if success, ID := disttrace.CheckSlaveAuth(db, slave.Name, slave.Secret); success == true {
		return true, ID
	}

	// no match found, unauthorized!
	log.Warnf("checkCredentials: Unauthorized slave '%v', peer: %v", slave.Name, req.RemoteAddr)
	time.Sleep(2 * time.Second)
	http.Error(writer, "Unauthorized", http.StatusUnauthorized)
	return false, uuid.Nil
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

		generateJSONResponse(writer, req, response)
	}
}

func httpHandleAPIGraphData() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		destID, err := uuid.Parse(req.URL.Query().Get("destID"))
		slaveID, err := uuid.Parse(req.URL.Query().Get("slaveID"))
		skip, _ := strconv.Atoi(req.URL.Query().Get("skip"))

		log.Debugf("httpHandleAPIGraphData: Received API 'graphdata' request, dest: <%v>, skip: <%v>, %v", destID, skip, err)

		if destID == uuid.Nil || slaveID == uuid.Nil {
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
				JOIN t_Slaves s ON t.strSlaveId = s.strSlaveId
				LEFT JOIN t_Hops prev ON h.strPreviousHopId = prev.strHopId

				WHERE tg.strTargetID = ? AND s.strSlaveId = ? AND h.nHopIndex > ?

				GROUP BY h.strHopIPAddress, h.nHopIndex, prevHopAddress
				ORDER BY h.nHopIndex
			) t
			GROUP BY t.strDestination
			`

		resRow := db.QueryRow(query, destID, slaveID, skip)
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
		SELECT t.strTracerouteId, s.strSlaveId, s.strSlaveName, tg.strTargetId, tg.strDestination, strftime("%d.%m.%Y %H:%M", t.dtStart) AS dtStart, COUNT(h.strHopId) AS nHopCount, 
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
			SlaveID    uuid.UUID
			SlaveName  string
			DestID     uuid.UUID
			DestName   string
			StartTime  string
			HopCnt     int64
			DetailJSON string
		}

		rows := []trace{}

		for resRows.Next() {
			var t trace
			if err = resRows.Scan(&t.TraceID, &t.SlaveID, &t.SlaveName, &t.DestID, &t.DestName, &t.StartTime, &t.HopCnt, &t.DetailJSON); err != nil {
				log.Warn("httpHandleAPITraceHistory: Couldn't read DB result set, Error: ", err)
				http.Error(writer, "Couldn't read DB result set", http.StatusInternalServerError)
				return
			}
			rows = append(rows, t)
		}

		generateJSONResponse(writer, req, rows)
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
		auth, slaveID := checkSlaveCredentials(&result.Slave, writer, req)
		if !auth {
			return
		}
		result.Slave.ID = slaveID

		// check data
		if target, err := disttrace.GetTarget(result.Target.ID, db); err != nil {
			log.Warnf("httpHandleSlaveResults: Couldn't get Target '%v', Error: %v", result.Target.ID, err)
			http.Error(writer, "Couldn't get Target", http.StatusInternalServerError)
			return
		} else if target.ID == uuid.Nil {
			log.Debug("httpHandleSlaveResults: Bogus result, Supplied target ID doesn't match a target in the DB, returning BadRequest")
			http.Error(writer, "Supplied target ID doesn't match a target in the DB", http.StatusBadRequest)
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

		generateJSONResponse(writer, req, response)
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
		auth, slaveID := checkSlaveCredentials(&slave, writer, req)
		if !auth {
			return
		}

		// read config from db
		slaveConf := disttrace.SlaveConfig{ID: slaveID}

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
	writer.Header().Add("Access-Control-Allow-Methods", "DELETE, POST, GET, OPTIONS, PUT")
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

func httpHandleAPISlavesList() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		log.Debug("httpHandleAPISlavesList: Received API 'slaves' request, method: ", req.Method)

		slaves, err := disttrace.GetSlaves(db)
		if err != nil {
			log.Warn("httpHandleAPISlavesList: Error: Couldn't get slaves from db, Error: ", err)
			http.Error(writer, "Couldn't get slaves from db", http.StatusInternalServerError)
			return
		}

		generateJSONResponse(writer, req, slaves)
	}
}

func httpHandleAPISlavesCreate() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		name := req.URL.Query().Get("name")
		secret := req.URL.Query().Get("secret")

		log.Debug("httpHandleAPISlavesCreate: Received API 'slaves' request, method: ", req.Method)

		if len(name) == 0 || len(secret) == 0 {
			log.Debugf("httpHandleAPISlavesCreate: Name: '%v' or secret: '%v' missing, returning bad request", name, secret)
			http.Error(writer, "name or secret missing", http.StatusBadRequest)
			return
		}

		slave := disttrace.Slave{
			Name:   name,
			Secret: secret,
		}

		newslave, err := disttrace.CreateSlave(db, slave)
		if err != nil {
			log.Warn("httpHandleAPISlavesCreate: Error while creating slave, Error: ", err)
			http.Error(writer, "Error while creating slave", http.StatusInternalServerError)
			return
		}

		// HTTP 201 Created
		writer.WriteHeader(201)
		generateJSONResponse(writer, req, newslave)
	}
}

func httpHandleAPISlavesUpdate() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		log.Debugf("httpHandleAPISlavesUpdate: Received API 'slaves' request, method: '%v'", req.Method)

		var slave disttrace.Slave
		decoder := json.NewDecoder(req.Body)
		if err := decoder.Decode(&slave); err != nil {
			log.Warn("httpHandleAPISlavesUpdate: Couldn't decode request body, Error: ", err)
			http.Error(writer, "Couldn't decode request body", http.StatusBadRequest)
			return
		}

		_, err := disttrace.UpdateSlave(db, slave)
		if err != nil {
			log.Warn("httpHandleAPISlavesUpdate: Error while updating slave, Error: ", err)
			http.Error(writer, "Error while updating slave", http.StatusInternalServerError)
			return
		}

		generateJSONResponse(writer, req, slave)
	}
}

func httpHandleAPISlavesDelete() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		log.Debugf("httpHandleAPISlavesDelete: Received API 'slaves' request, method: '%v', ID: '%v'", req.Method, vars["slaveID"])

		slaveID, err := uuid.Parse(vars["slaveID"])
		if err != nil {
			log.Debugf("httpHandleAPISlavesDelete: Received delete request for invalid slave, ID: '%v', Error: %v", slaveID, err)
			http.Error(writer, "Received delete request for invalid slave", http.StatusBadRequest)
			return
		}

		if err = disttrace.DeleteSlave(db, slaveID); err != nil {
			log.Warn("httpHandleAPISlavesDelete: Error while deleting slave, Error: ", err)
			http.Error(writer, "Error while deleting slave", http.StatusInternalServerError)
			return
		}

		generateJSONResponse(writer, req, disttrace.Slave{ID: slaveID})
	}
}

func httpHandleAPITargetsList() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		log.Debug("httpHandleAPITargetsList: Received API 'targets' request, method: ", req.Method)

		targets, err := disttrace.GetTargets(db)
		if err != nil {
			log.Warn("httpHandleAPITargetsList: Error: Couldn't get targets from db, Error: ", err)
			http.Error(writer, "Couldn't get targets from db", http.StatusInternalServerError)
			return
		}

		generateJSONResponse(writer, req, targets)
	}
}

func httpHandleAPITargetsCreate() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		name := req.URL.Query().Get("name")
		address := req.URL.Query().Get("address")
		retries, _ := strconv.Atoi(req.URL.Query().Get("retries"))
		maxHops, _ := strconv.Atoi(req.URL.Query().Get("maxHops"))
		timeout, _ := strconv.Atoi(req.URL.Query().Get("timeout"))

		log.Debug("httpHandleAPITargetsCreate: Received API 'targets' request, method: ", req.Method)

		if len(name) == 0 || len(address) == 0 {
			log.Debugf("httpHandleAPITargetsCreate: Name: '%v' or address: '%v' missing, returning bad request", name, address)
			http.Error(writer, "name or address missing", http.StatusBadRequest)
			return
		}

		target := disttrace.TraceTarget{
			Name:      name,
			Address:   address,
			Retries:   retries,
			MaxHops:   maxHops,
			TimeoutMs: timeout,
		}

		newTarget, err := disttrace.CreateTarget(db, target)
		if err != nil {
			log.Warn("httpHandleAPITargetsCreate: Error while creating target, Error: ", err)
			http.Error(writer, "Error while creating target", http.StatusInternalServerError)
			return
		}

		// HTTP 201 Created
		writer.WriteHeader(201)
		generateJSONResponse(writer, req, newTarget)
	}
}

func httpHandleAPITargetsUpdate() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		log.Debugf("httpHandleAPITargetsUpdate: Received API 'targets' request, method: '%v'", req.Method)

		var target disttrace.TraceTarget
		decoder := json.NewDecoder(req.Body)
		if err := decoder.Decode(&target); err != nil {
			log.Warn("httpHandleAPITargetsUpdate: Couldn't decode request body, Error: ", err)
			http.Error(writer, "Couldn't decode request body", http.StatusBadRequest)
			return
		}

		_, err := disttrace.UpdateTarget(db, target)
		if err != nil {
			log.Warn("httpHandleAPITargetsUpdate: Error while updating target, Error: ", err)
			http.Error(writer, "Error while updating target", http.StatusInternalServerError)
			return
		}

		generateJSONResponse(writer, req, target)
	}
}

func httpHandleAPITargetsDelete() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		log.Debugf("httpHandleAPITargetsDelete: Received API 'targets' request, method: '%v', ID: '%v'", req.Method, vars["targetID"])

		targetID, err := uuid.Parse(vars["targetID"])
		if err != nil {
			log.Debugf("httpHandleAPITargetsDelete: Received delete request for invalid target, ID: '%v', Error: %v", targetID, err)
			http.Error(writer, "Received delete request for invalid target", http.StatusBadRequest)
			return
		}

		if err = disttrace.DeleteTarget(db, targetID); err != nil {
			log.Warn("httpHandleAPITargetsDelete: Error while deleting target, Error: ", err)
			http.Error(writer, "Error while deleting target", http.StatusInternalServerError)
			return
		}

		generateJSONResponse(writer, req, disttrace.TraceTarget{ID: targetID})
	}
}

func httpHandleAPIUsersList() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		log.Debug("httpHandleAPIUsersList: Received API 'users' request, method: ", req.Method)

		users, err := disttrace.GetUsers(db)
		if err != nil {
			log.Warn("httpHandleAPIUsersList: Error: Couldn't get users from db, Error: ", err)
			http.Error(writer, "Couldn't get users from db", http.StatusInternalServerError)
			return
		}

		generateJSONResponse(writer, req, users)
	}
}

func httpHandleAPIUsersCreate() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		name := req.URL.Query().Get("name")
		password := req.URL.Query().Get("password")
		pwNeedsChange, _ := strconv.ParseBool(req.URL.Query().Get("pwNeedsChange"))

		log.Debug("httpHandleAPIUsersCreate: Received API 'users' request, method: ", req.Method)

		if len(name) == 0 || len(password) == 0 {
			log.Debugf("httpHandleAPIUsersCreate: Name: '%v' or address: '%v' missing, returning bad request", name, password)
			http.Error(writer, "name or password missing", http.StatusBadRequest)
			return
		}

		user := disttrace.User{
			Name:                name,
			Password:            password,
			PasswordNeedsChange: pwNeedsChange,
		}

		newUser, err := disttrace.CreateUser(db, user)
		if err != nil {
			log.Warn("httpHandleAPIUsersCreate: Error while creating user, Error: ", err)
			http.Error(writer, "Error while creating user", http.StatusInternalServerError)
			return
		}

		// HTTP 201 Created
		writer.WriteHeader(201)
		generateJSONResponse(writer, req, newUser)
	}
}

func httpHandleAPIUsersUpdate() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		log.Debugf("httpHandleAPIUsersUpdate: Received API 'users' request, method: '%v'", req.Method)

		var user disttrace.User
		decoder := json.NewDecoder(req.Body)
		if err := decoder.Decode(&user); err != nil {
			log.Warn("httpHandleAPIUsersUpdate: Couldn't decode request body, Error: ", err)
			http.Error(writer, "Couldn't decode request body", http.StatusBadRequest)
			return
		}

		_, err := disttrace.UpdateUser(db, user)
		if err != nil {
			log.Warn("httpHandleAPIUsersUpdate: Error while updating user, Error: ", err)
			http.Error(writer, "Error while updating user", http.StatusInternalServerError)
			return
		}

		generateJSONResponse(writer, req, user)
	}
}

func httpHandleAPIUsersDelete() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		log.Debugf("httpHandleAPIUsersDelete: Received API 'users' request, method: '%v', ID: '%v'", req.Method, vars["userID"])

		userID, err := uuid.Parse(vars["userID"])
		if err != nil {
			log.Debugf("httpHandleAPIUsersDelete: Received delete request for invalid user, ID: '%v', Error: %v", userID, err)
			http.Error(writer, "Received delete request for invalid user", http.StatusBadRequest)
			return
		}

		if err = disttrace.DeleteUser(db, userID); err != nil {
			log.Warn("httpHandleAPIUsersDelete: Error while deleting user, Error: ", err)
			http.Error(writer, "Error while deleting user", http.StatusInternalServerError)
			return
		}

		generateJSONResponse(writer, req, disttrace.TraceTarget{ID: userID})
	}
}

// generateJSONResponse takes an object, and returns it as JSON object in the response body
func generateJSONResponse(writer http.ResponseWriter, req *http.Request, val interface{}) {
	json, err := json.MarshalIndent(val, "", "	")
	if err != nil {
		log.Warn("generateJSONResponse: Error: marshal slaves into json, Error: ", err)
		http.Error(writer, "Couldn't marshal data into json", http.StatusInternalServerError)
		return
	}

	_, err = writer.Write(json)
	if err != nil {
		log.Warn("generateJSONResponse: Couldn't write response: ", err)
		return
	}

	log.Debugf("generateJSONResponse: Successfully finished...")
	return
}
