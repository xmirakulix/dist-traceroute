package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"time"

	ghandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"github.com/urfave/negroni"
	"github.com/xmirakulix/dist-traceroute/disttrace"
)

var httpProcQuitDone = make(chan bool, 1)

func httpServer(accessLog string) {
	var err error

	log.Info("httpServer: Start...")

	var accessWriter io.Writer
	if accessWriter, err = os.OpenFile(accessLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err != nil {
		log.Panicf("httpServer: Can't open access log '%v', Error: %v", accessLog, err)
	}

	// handle slaves
	slaveRouter := mux.NewRouter()
	slaveRouter.HandleFunc("/slave/results", httpHandleSlaveResults())
	slaveRouter.HandleFunc("/slave/config", httpHandleSlaveConfig())

	// handle api requests from webinterface
	apiRouter := mux.NewRouter()
	apiRouter.HandleFunc("/api/status", httpHandleAPIStatus())
	apiRouter.HandleFunc("/api/traces", httpHandleAPITraceHistory())
	apiRouter.HandleFunc("/api/graph", httpHandleAPIGraphData())

	apiRouter.HandleFunc("/api/slaves", httpHandleAPISlavesList()).Methods("GET")
	apiRouter.HandleFunc("/api/slaves", httpHandleAPISlavesCreate()).Methods("POST")
	apiRouter.HandleFunc("/api/slaves", httpHandleAPISlavesUpdate()).Methods("PUT")
	apiRouter.HandleFunc("/api/slaves/{slaveID}", httpHandleAPISlavesDelete()).Methods("DELETE")

	apiRouter.HandleFunc("/api/users", httpHandleAPIUsersList()).Methods("GET")
	apiRouter.HandleFunc("/api/users", httpHandleAPIUsersCreate()).Methods("POST")
	apiRouter.HandleFunc("/api/users", httpHandleAPIUsersUpdate()).Methods("PUT")
	apiRouter.HandleFunc("/api/users/{userID}", httpHandleAPIUsersDelete()).Methods("DELETE")

	apiRouter.HandleFunc("/api/targets", httpHandleAPITargetsList()).Methods("GET")
	apiRouter.HandleFunc("/api/targets", httpHandleAPITargetsCreate()).Methods("POST")
	apiRouter.HandleFunc("/api/targets", httpHandleAPITargetsUpdate()).Methods("PUT")
	apiRouter.HandleFunc("/api/targets/{targetID}", httpHandleAPITargetsDelete()).Methods("DELETE")

	authHandler := negroni.New()
	authHandler.Use(negroni.HandlerFunc(checkJWTAuth))
	authHandler.UseHandler(apiRouter)

	// handle everything else
	rootRouter := http.NewServeMux()
	rootRouter.HandleFunc("/", httpDefaultHandler())
	rootRouter.HandleFunc("/api/auth", httpHandleAPIAuth())
	rootRouter.Handle("/slave/", slaveRouter)
	rootRouter.Handle("/api/", authHandler)

	// register middleware for all requests
	rootHandler := negroni.New()
	rootHandler.Use(negroni.HandlerFunc(handleAccessControl))
	rootHandler.Use(negroni.Wrap(ghandlers.CombinedLoggingHandler(accessWriter, rootRouter)))

	srv := &http.Server{
		Addr:         ":8990",
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      rootHandler,
	}

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
