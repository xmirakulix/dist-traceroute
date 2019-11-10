package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	valid "github.com/asaskevich/govalidator"

	ghandlers "github.com/gorilla/handlers"
	"github.com/sirupsen/logrus"
	"github.com/xmirakulix/dist-traceroute/disttrace"
)

// TODO log results to seperate log
// TODO add option to post results to elastic
// TODO https/TLS
// TODO fix multiline traces when logging to logfile (e.g. cmdline arg usage)
// TODO config of slaves and targets in db

// global logger
var log = logrus.New()

var httpProcQuitDone = make(chan bool, 1)

// status vars for webinterface
var lastTransmittedSlaveConfig = "none yet"
var lastTransmittedSlaveConfigTime time.Time

var db *disttrace.DB

func checkCredentials(slaveCreds disttrace.SlaveCredentials, writer http.ResponseWriter, req *http.Request, ppCfg **disttrace.GenericConfig) (success bool) {

	success = false
	pCfg := *ppCfg

	// check for match in master config
	for _, trustedSlave := range pCfg.Slaves {
		if trustedSlave.Name == slaveCreds.Name && trustedSlave.Password == slaveCreds.Password {

			// success!
			log.Debugf("checkCredentials: Successfully authenticated slave '%v' from peer: %v", slaveCreds.Name, req.RemoteAddr)
			return true
		}
	}

	// no match found, unauthorized!
	log.Warnf("checkCredentials: Unauthorized slave '%v', peer: %v", slaveCreds.Name, req.RemoteAddr)
	time.Sleep(2 * time.Second)
	http.Error(writer, "Unauthorized", http.StatusUnauthorized)
	return false
}

func httpAPIHandlerStatus(ppCfg **disttrace.GenericConfig) http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		log.Debug("httpDefaultHandler: Received API 'status' request")

		var timeSinceSlaveCfg string
		if lastTransmittedSlaveConfigTime.IsZero() {
			timeSinceSlaveCfg = ""
		} else {
			timeSinceSlaveCfg = time.Since(lastTransmittedSlaveConfigTime).Truncate(time.Second).String() + " ago"
		}

		pCfg := *ppCfg
		response := struct {
			Uptime              string
			CurrentMasterConfig disttrace.MasterConfig
			LastSlaveConfigTime string
			LastSlaveConfig     string
		}{
			disttrace.GetUptime().Truncate(time.Second).String(),
			*pCfg.MasterConfig,
			timeSinceSlaveCfg,
			lastTransmittedSlaveConfig,
		}

		resJSON, _ := json.MarshalIndent(response, "", "	")

		writer.Header().Add("Access-Control-Allow-Origin", "*")
		if _, err := writer.Write(resJSON); err != nil {
			log.Warn("httpDefaultHandler: Couldn't write response: ", err)
		}

	}
}

func httpAPIHandlerTraces(ppCfg **disttrace.GenericConfig) http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		log.Debugf("httpDefaultHandler: Received API 'status' request, limit: <%v>", req.URL.Query().Get("_limit"))

		limit, _ := strconv.Atoi(req.URL.Query().Get("_limit"))

		lastResultsQuery := `
		SELECT t.nTracerouteId, t.strOriginSlave, t.strDestination, strftime("%d.%m.%Y %H:%M", t.dtStart) AS dtStart, COUNT(h.nHopId) AS nHopCount, 
			json_group_object(h.nHopIndex, json_object('IP', h.strHopIPAddress, 'DNS', h.strHopDNSName, 'Duration', h.dDurationSec)) AS strHopDetails
		FROM t_Traceroutes t LEFT JOIN t_Hops h ON t.nTracerouteId = h.nTracerouteId
		GROUP BY t.nTracerouteId
		ORDER BY t.nTracerouteId DESC
		`

		if limit != 0 {
			lastResultsQuery += "LIMIT " + strconv.Itoa(limit)
		}

		var resRows *sql.Rows
		var err error

		if resRows, err = db.Query(lastResultsQuery); err != nil {
			log.Warn("httpDefaultHandler: Couldn't get last results from DB, Error: ", err)
			http.Error(writer, "Couldn't get last results from DB", http.StatusInternalServerError)
			return
		}

		type trace struct {
			TraceID    int64
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
				log.Warn("httpDefaultHandler: Couldn't read DB result set, Error: ", err)
				http.Error(writer, "Couldn't read DB result set", http.StatusInternalServerError)
				return
			}
			rows = append(rows, t)
		}

		resRows.Close()

		var response []byte
		if response, err = json.MarshalIndent(rows, "", "	"); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}

		writer.Header().Add("Access-Control-Allow-Origin", "*")
		if _, err := writer.Write(response); err != nil {
			log.Warn("httpDefaultHandler: Couldn't write response: ", err)
		}
	}
}

func httpDefaultHandler(ppCfg **disttrace.GenericConfig) http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		log.Info("httpDefaultHandler: Received request for base/unknown URL, returning 'Not Found': ", req.URL)

		// reply with error
		http.Error(writer, "Not found", http.StatusNotFound)
		return
	}
}

func httpRxResultHandler(ppCfg **disttrace.GenericConfig) http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		log.Debug("httpRxResultHandler: Received request results, URL: ", req.URL)

		// init vars
		result := disttrace.TraceResult{}
		jsonDecoder := json.NewDecoder(req.Body)

		// decode request
		err := jsonDecoder.Decode(&result)
		if err != nil {
			log.Warn("httpRxResultHandler: Couldn't decode request body into JSON: ", err)

			// create error response
			response := disttrace.SubmitResult{
				Success:       false,
				Error:         "Couldn't decode request body into JSON: " + err.Error(),
				RetryPossible: false,
			}

			var responseJSON []byte
			if responseJSON, err = json.Marshal(response); err != nil {
				http.Error(writer, "Error: Couldn't marshal error response into JSON", http.StatusBadRequest)
				log.Warn("httpRxResultHandler: Error: Couldn't marshal error response into JSON: ", err)
				return
			}

			// reply with error
			http.Error(writer, string(responseJSON), http.StatusBadRequest)
			return
		}

		// check authorization
		if !checkCredentials(result.Creds, writer, req, ppCfg) {
			return
		}

		log.Infof("httpRxResultHandler: Received results from slave '%v' for target '%v'. Success: %v, Hops: %v.",
			result.Creds.Name, result.Target.Name,
			result.Success, result.HopCount,
		)

		if ok, e := disttrace.ValidateTraceResult(result); !ok || e != nil {
			log.Warn("httpRxResultHandler: Result validation failed, Error: ", e)
			http.Error(writer, "Result validation failed: "+e.Error(), http.StatusBadRequest)
			return
		}

		// store submitted result
		var tx *disttrace.Tx
		var errDb error
		errDb = nil

		if tx, errDb = db.Begin(); err != nil {
			log.Warn("httpRxResultHandler: Error creating database transaction while storing result, Error: ", errDb)
			http.Error(writer, "Database error", http.StatusInternalServerError)
			return
		}
		// catch errors and rollback!
		defer func() {
			if errDb != nil {
				log.Warn("httpRxResultHandler: Caught error during database operations, rolling transaction back!")
				tx.Rollback()
			}
		}()

		// prepare traceroute insert
		traceStmt, errDb := tx.Prepare(`
			INSERT INTO t_Traceroutes (nTracerouteId, strOriginSlave, strDestination, dtStart, strAnnotations) 
			VALUES (?, ?, ?, ?, ?)
			`)
		defer traceStmt.Close()
		if errDb != nil {
			log.Warn("httpRxResultHandler: Error while preparing database statement, Error: ", errDb)
			http.Error(writer, "Database error", http.StatusInternalServerError)
			return
		}

		// prepare hop insert
		hopStmt, errDb := tx.Prepare(`
			INSERT INTO t_Hops (nHopId, nTracerouteId, nHopIndex, strHopIPAddress, strHopDNSName, dDurationSec, nPreviousHopId)	
			VALUES (?, ?, ?, ?, ?, ?, ?) 
			`)
		defer hopStmt.Close()
		if errDb != nil {
			log.Warn("httpRxResultHandler: Error while preparing database statement, Error: ", errDb)
			http.Error(writer, "Database error", http.StatusInternalServerError)
			return
		}

		log.Debug("httpRxResultHandler: Finished preparing queries, inserting data...")

		// Insert result info
		resTrace, errDb := traceStmt.Exec(nil, result.Creds.Name, result.Target.Address, result.DateTime.Format(time.RFC3339), "")
		if errDb != nil {
			log.Warn("httpRxResultHandler: Error while inserting result, Error: ", errDb)
			http.Error(writer, "Database error", http.StatusInternalServerError)
			return
		}
		lastTraceID, errDb := resTrace.LastInsertId()
		if errDb != nil {
			log.Warn("httpRxResultHandler: Error while getting last inserted ID of traceroute, Error: ", errDb)
			http.Error(writer, "Database error", http.StatusInternalServerError)
			return
		}
		log.Debug("httpRxResultHandler: Inserted result with ID: ", lastTraceID)

		// Insert hops info
		var prevHopID int64
		for _, hop := range result.Hops {
			var resHop sql.Result
			// prev hop is null on first hop
			if hop.TTL == 0 {
				resHop, errDb = hopStmt.Exec(nil, lastTraceID, hop.TTL, hop.AddressString(), hop.Host, hop.ElapsedTime.Seconds(), nil)
			} else {
				resHop, errDb = hopStmt.Exec(nil, lastTraceID, hop.TTL, hop.AddressString(), hop.Host, hop.ElapsedTime.Seconds(), prevHopID)
			}
			if errDb != nil {
				log.Warn("httpRxResultHandler: Error while inserting hop, Error: ", errDb)
				http.Error(writer, "Database error", http.StatusInternalServerError)
				return
			}
			prevHopID, errDb = resHop.LastInsertId()
			if errDb != nil {
				log.Warn("httpRxResultHandler: Error while getting last inserted ID of hop, Error: ", errDb)
				http.Error(writer, "Database error", http.StatusInternalServerError)
				return
			}
		}
		log.Debug("httpRxResultHandler: Successfully inserted trace info and hops, commiting transaction...")

		if errDb = tx.Commit(); errDb != nil {
			log.Warn("httpRxResultHandler: Error while commiting transaction, Error: ", errDb)
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
			log.Warn("httpRxResultHandler: Error: Couldn't marshal success response into JSON: ", err)
			return
		}

		// Success!
		_, err = io.WriteString(writer, string(responseJSON))
		if err != nil {
			log.Warn("httpRxResultHandler: Couldn't write success response: ", err)
		}
		log.Debug("httpRxResultHandler: Replying success.")
		return
	}
}

func httpTxConfigHandler(targetConfigFile string, ppCfg **disttrace.GenericConfig) http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		log.Debug("httpTxConfigHandler: Received request for config, URL: ", req.URL)

		var err error

		// read request body
		var reqBody []byte
		if reqBody, err = ioutil.ReadAll(req.Body); err != nil {
			log.Warn("httpTxConfigHandler: Can't read request body, Error: ", err)
			http.Error(writer, "Can't read request", http.StatusInternalServerError)
			return
		}

		// parse JSON from request body
		var slaveCreds disttrace.SlaveCredentials
		if err = json.Unmarshal(reqBody, &slaveCreds); err != nil {
			log.Warn("httpTxConfigHandler: Can't unmarshal request body into slave creds, Error: ", err)
			http.Error(writer, "Can't unmarshal request body", http.StatusBadRequest)
			return
		}

		// check authorization
		if !checkCredentials(slaveCreds, writer, req, ppCfg) {
			return
		}

		// read config from disk
		var body []byte
		if body, err = ioutil.ReadFile(targetConfigFile); err != nil {
			http.Error(writer, "Error: Couldn't read config file!", http.StatusInternalServerError)
			log.Warn("httpTxConfigHandler: Error: Couldn't read config file: ", err)
			lastTransmittedSlaveConfig = "Error: Couldn't read config file: " + err.Error()
			lastTransmittedSlaveConfigTime = time.Now()
			return
		}

		slaveConf := disttrace.SlaveConfig{}

		// check if file can be parsed
		if err = json.Unmarshal(body, &slaveConf); err != nil {
			http.Error(writer, "Error: Can't parse config", http.StatusInternalServerError)
			log.Warn("httpTxConfigHandler: Loaded config can't be parsed, Error: ", err)
			lastTransmittedSlaveConfig = "Error: Can't parse config: " + err.Error()
			lastTransmittedSlaveConfigTime = time.Now()
			return
		}

		// validate config
		if ok, e := valid.ValidateStruct(slaveConf); !ok || e != nil {
			http.Error(writer, "Error: Loaded config is invalid", http.StatusInternalServerError)
			log.Warn("httpTxConfigHandler: Loaded config is invalid, Error: ", e)
			lastTransmittedSlaveConfig = "Error: Loaded config is invalid: " + err.Error()
			lastTransmittedSlaveConfigTime = time.Now()
			return
		}

		// send config to slave
		lastTransmittedSlaveConfig = string(body)
		lastTransmittedSlaveConfigTime = time.Now()
		_, err = io.WriteString(writer, string(body))
		if err != nil {
			log.Warn("httpTxConfigHandler: Couldn't write success response: ", err)
			return
		}

		log.Infof("httpTxConfigHandler: Transmitting currently configured targets to slave '%v' for %v targets", slaveCreds.Name, len(slaveConf.Targets))
		return
	}
}

func writeAccessLog(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}

func httpServer(ppCfg **disttrace.GenericConfig, accessLog string, targetConfigFile string) {
	var err error

	log.Info("httpServer: Start...")

	disttrace.WaitForValidConfig("httpServer", "master", ppCfg)

	var accessWriter io.Writer
	if accessWriter, err = os.OpenFile(accessLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err != nil {
		log.Panicf("httpServer: Can't open access log '%v', Error: %v", accessLog, err)
	}

	router := http.NewServeMux()
	srv := &http.Server{
		Addr:    ":8990",
		Handler: ghandlers.CombinedLoggingHandler(accessWriter, router),
	}

	// handle results from slaves
	router.HandleFunc("/results/", httpRxResultHandler(ppCfg))

	// handle config requests from slaves
	router.HandleFunc("/config/", httpTxConfigHandler(targetConfigFile, ppCfg))

	// handle api requests from webinterface
	router.HandleFunc("/api/status", httpAPIHandlerStatus(ppCfg))
	router.HandleFunc("/api/traces", httpAPIHandlerTraces(ppCfg))

	// handle everything else
	router.HandleFunc("/", httpDefaultHandler(ppCfg))

	// start server...
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal("httpServer: HTTP Server failure, ListenAndServe: ", err)
		}
	}()

	// wait for quit signal...
	for {
		if disttrace.CheckForQuit() {
			log.Warn("httpServer: Received signal to shutdown...")
			ctx, cFunc := context.WithTimeout(context.Background(), 5*time.Second)
			if err := srv.Shutdown(ctx); err != nil {
				log.Warn("httpServer: Error while shutdown of HTTP server, Error: ", err)
			}
			cFunc()

			log.Info("httpServer: Shutdown complete.")
			httpProcQuitDone <- true
			return
		}

		time.Sleep(1 * time.Second)
	}
}

func main() {

	// parse cmdline arguments
	var masterConfigNameAndPath, targetsConfigNameAndPath string
	var mainLogNameAndPath, accessLogNameAndPath string
	var dbNameAndPath string
	var logLevel string

	// check cmdline args
	{
		var sendHelp bool

		fSet := flag.FlagSet{}
		outBuf := bytes.NewBuffer([]byte{})
		fSet.SetOutput(outBuf)
		fSet.StringVar(&masterConfigNameAndPath, "slaves", "./dt-slaves.json", "Set config `filename`")
		fSet.StringVar(&targetsConfigNameAndPath, "targets", "./dt-targets.json", "Set config `filename`")
		fSet.StringVar(&dbNameAndPath, "db", "./disttrace.db", "Set database `filename`")
		fSet.StringVar(&mainLogNameAndPath, "log", "./dt-master.log", "Main logfile location `/path/to/file`")
		fSet.StringVar(&accessLogNameAndPath, "accesslog", "./dt-access.log", "HTTP access logfile location `/path/to/file`")
		fSet.StringVar(&logLevel, "loglevel", "info", "Specify loglevel, one of `warn, info, debug`")
		fSet.BoolVar(&sendHelp, "help", false, "display this message")
		fSet.Parse(os.Args[1:])

		var errMasterCfg, errTargetsCfg, errMainLog, errAccessLog, errDb error
		masterConfigNameAndPath, errMasterCfg = disttrace.CleanAndCheckFileNameAndPath(masterConfigNameAndPath)
		targetsConfigNameAndPath, errTargetsCfg = disttrace.CleanAndCheckFileNameAndPath(targetsConfigNameAndPath)
		mainLogNameAndPath, errMainLog = disttrace.CleanAndCheckFileNameAndPath(mainLogNameAndPath)
		accessLogNameAndPath, errAccessLog = disttrace.CleanAndCheckFileNameAndPath(accessLogNameAndPath)
		dbNameAndPath, errDb = disttrace.CleanAndCheckFileNameAndPath(dbNameAndPath)

		// valid cmdline arguments or exit
		switch {
		case errMasterCfg != nil || errTargetsCfg != nil:
			log.Warn("Error: Invalid config file name, can't run, Bye.")
			disttrace.PrintMasterUsageAndExit(fSet, true)
		case errMainLog != nil || errAccessLog != nil:
			log.Warn("Error: Invalid log path specified, can't run, Bye.")
			disttrace.PrintMasterUsageAndExit(fSet, true)
		case errDb != nil:
			log.Warn("Error: Invalid database path specified, can't run, Bye.")
			disttrace.PrintMasterUsageAndExit(fSet, true)
		case logLevel != "warn" && logLevel != "info" && logLevel != "debug":
			log.Warn("Error: Invalid loglevel specified, can't run, Bye.")
			disttrace.PrintMasterUsageAndExit(fSet, true)
		case sendHelp:
			disttrace.PrintMasterUsageAndExit(fSet, false)
		}
	}

	// setup logging
	disttrace.SetLogOptions(log, mainLogNameAndPath, logLevel)

	// let's Go! :)
	log.Warn("Main: Starting...")
	disttrace.DebugPrintAllArguments(masterConfigNameAndPath, mainLogNameAndPath, logLevel)

	// setup listener for OS exit signals
	disttrace.ListenForOSSignals()

	// create master configuration
	var pCfg = new(disttrace.GenericConfig)
	pCfg.MasterConfig = new(disttrace.MasterConfig)
	var ppCfg = &pCfg

	// init database connection
	{
		var err error
		if db, err = disttrace.InitDBConnectionAndUpdate(dbNameAndPath); err != nil {
			log.Fatal("Main: Couldn't initiate database connection! Error: ", err)
		}
		log.Info("Main: Database connection initiated...")
	}

	log.Info("Main: Launching config poller process...")
	go disttrace.MasterConfigPoller(masterConfigNameAndPath, ppCfg)

	log.Info("Main: Launching http server process...")
	go httpServer(ppCfg, accessLogNameAndPath, targetsConfigNameAndPath)

	// wait here until told to quit by os signal
	log.Info("Main: startup finished, going to sleep...")
	disttrace.WaitForOSSignalAndQuit()

	// wait for graceful shutdown of HTTP server
	log.Info("Main: waiting for HTTP server shutdown...")
	<-httpProcQuitDone

	log.Warn("Main: Everything has gracefully ended...")
	log.Warn("Main: Bye.")
}
