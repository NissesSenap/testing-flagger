/*
Copyright 2020 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package loadtester

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/sethvargo/go-limiter"
	"github.com/slok/go-http-metrics/middleware"
	"github.com/slok/go-http-metrics/middleware/std"
	"sigs.k8s.io/controller-runtime/pkg/client"

	flaggerv1 "github.com/fluxcd/flagger/pkg/apis/flagger/v1beta1"
	"github.com/fluxcd/flagger/pkg/logger"
	"github.com/fluxcd/pkg/runtime/events"
)

// EventServer handles event POST requests
type EventServer struct {
	port       string
	logger     logr.Logger
	kubeClient client.Client
}

// NewEventServer returns an HTTP server that handles events
func NewEventServer(port string, logger logr.Logger, kubeClient client.Client) *EventServer {
	return &EventServer{
		port:       port,
		logger:     logger.WithName("event-server"),
		kubeClient: kubeClient,
	}
}

// ListenAndServe starts the HTTP server on the specified port
func (s *EventServer) ListenAndServe(stopCh <-chan struct{}, mdlw middleware.Middleware, store limiter.Store, taskRunner *TaskRunner, gate *GateStorage) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", HandleHealthz)
	mux.HandleFunc("/gate/approve", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("/gate/halt", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Forbidden"))
	})
	mux.HandleFunc("/gate/check", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			s.logger.Error(err, "reading the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		canary := &flaggerv1.CanaryWebhookPayload{}
		err = json.Unmarshal(body, canary)
		if err != nil {
			s.logger.Error(err, "decoding the request body failed %v")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		canaryName := fmt.Sprintf("%s.%s", canary.Name, canary.Namespace)
		approved := gate.isOpen(canaryName)
		if approved {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Approved"))
		} else {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Forbidden"))
		}

		s.logger.Info("%s gate check: approved %v", canaryName, approved)
	})

	mux.HandleFunc("/gate/open", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			s.logger.Error(err, "reading the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		canary := &flaggerv1.CanaryWebhookPayload{}
		err = json.Unmarshal(body, canary)
		if err != nil {
			s.logger.Error(err, "decoding the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		canaryName := fmt.Sprintf("%s.%s", canary.Name, canary.Namespace)
		gate.open(canaryName)

		w.WriteHeader(http.StatusAccepted)

		s.logger.Info("%s gate opened", canaryName)
	})

	mux.HandleFunc("/gate/close", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			s.logger.Error(err, "reading the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		canary := &flaggerv1.CanaryWebhookPayload{}
		err = json.Unmarshal(body, canary)
		if err != nil {
			s.logger.Error(err, "decoding the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		canaryName := fmt.Sprintf("%s.%s", canary.Name, canary.Namespace)
		gate.close(canaryName)

		w.WriteHeader(http.StatusAccepted)

		s.logger.Info("%s gate closed", canaryName)
	})

	mux.HandleFunc("/rollback/check", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			s.logger.Error(err, "reading the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		canary := &flaggerv1.CanaryWebhookPayload{}
		err = json.Unmarshal(body, canary)
		if err != nil {
			s.logger.Error(err, "decoding the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		canaryName := fmt.Sprintf("rollback.%s.%s", canary.Name, canary.Namespace)
		approved := gate.isOpen(canaryName)
		if approved {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Approved"))
		} else {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Forbidden"))
		}

		s.logger.Info("%s rollback check: approved %v", canaryName, approved)
	})
	mux.HandleFunc("/rollback/open", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			s.logger.Error(err, "reading the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		canary := &flaggerv1.CanaryWebhookPayload{}
		err = json.Unmarshal(body, canary)
		if err != nil {
			s.logger.Error(err, "decoding the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		canaryName := fmt.Sprintf("rollback.%s.%s", canary.Name, canary.Namespace)
		gate.open(canaryName)

		w.WriteHeader(http.StatusAccepted)

		s.logger.Info("%s rollback opened", canaryName)
	})
	mux.HandleFunc("/rollback/close", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			s.logger.Error(err, "reading the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		canary := &flaggerv1.CanaryWebhookPayload{}
		err = json.Unmarshal(body, canary)
		if err != nil {
			s.logger.Error(err, "decoding the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		canaryName := fmt.Sprintf("rollback.%s.%s", canary.Name, canary.Namespace)
		gate.close(canaryName)

		w.WriteHeader(http.StatusAccepted)

		s.logger.Info("%s rollback closed", canaryName)
	})
	//mux.Handle("/", s.logRateLimitMiddleware(limitMiddleware.Handle(http.HandlerFunc(s.handleEvent()))))

	// TODO(Edvin) change the log framework used
	zaplogger, err := logger.NewLoggerWithEncoding("debug", "json")
	if err != nil {
		log.Fatalf("Error creating logger: %v", err)
	}

	mux.HandleFunc("/", HandleNewTask(zaplogger, taskRunner, s.kubeClient))
	h := std.Handler("", mdlw, mux)
	srv := &http.Server{
		Addr:    s.port,
		Handler: h,
	}

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			s.logger.Error(err, "Event server crashed")
			os.Exit(1)
		}
	}()

	// wait for SIGTERM or SIGINT
	<-stopCh
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		s.logger.Error(err, "Event server graceful shutdown failed")
	} else {
		s.logger.Info("Event server stopped")
	}
}

type statusRecorder struct {
	http.ResponseWriter
	Status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.Status = status
	r.ResponseWriter.WriteHeader(status)
}

// HandleHealthz handles heath check requests
func HandleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *EventServer) logRateLimitMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{
			ResponseWriter: w,
			Status:         200,
		}
		h.ServeHTTP(recorder, r)

		if recorder.Status == http.StatusTooManyRequests {
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				s.logger.Error(err, "reading the request body failed")
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			event := &events.Event{}
			err = json.Unmarshal(body, event)
			if err != nil {
				s.logger.Error(err, "decoding the request body failed")
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			r.Body = ioutil.NopCloser(bytes.NewBuffer(body))

			s.logger.V(1).Info("Discarding event, rate limiting duplicate events",
				"reconciler kind", event.InvolvedObject.Kind,
				"name", event.InvolvedObject.Name,
				"namespace", event.InvolvedObject.Namespace)
		}
	})
}
