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
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	flaggerv1 "github.com/fluxcd/flagger/pkg/apis/flagger/v1beta1"
	"go.uber.org/zap"
)

// HandleNewTask handles task creation requests
func HandleNewTask(logger *zap.SugaredLogger, taskRunner TaskRunnerInterface) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Error("reading the request body failed", zap.Error(err))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		payload := &flaggerv1.CanaryWebhookPayload{}
		err = json.Unmarshal(body, payload)
		if err != nil {
			logger.Error("decoding the request body failed", zap.Error(err))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if len(payload.Metadata) > 0 {
			metadata := payload.Metadata
			var typ, ok = metadata["type"]
			if !ok {
				typ = TaskTypeShell
			}

			rtnCmdOutput := false
			if rtn, ok := metadata["returnCmdOutput"]; ok {
				rtnCmdOutput, err = strconv.ParseBool(rtn)
			}

			// run bash command (blocking task)
			if typ == TaskTypeBash {
				logger.With("canary", payload.Name).Infof("bash command %s", payload.Metadata["cmd"])

				bashTask := BashTask{
					command:      payload.Metadata["cmd"],
					logCmdOutput: true,
					TaskBase: TaskBase{
						canary: fmt.Sprintf("%s.%s", payload.Name, payload.Namespace),
						logger: logger,
					},
				}

				ctx, cancel := context.WithTimeout(context.Background(), taskRunner.Timeout())
				defer cancel()

				result, err := bashTask.Run(ctx)
				if !result.ok {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(err.Error()))
					return
				}

				w.WriteHeader(http.StatusOK)
				if rtnCmdOutput {
					w.Write(result.out)
				}
				return
			}

			taskFactory, ok := GetTaskFactory(typ)
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(fmt.Sprintf("unknown task type %s", typ)))
				return
			}
			canary := fmt.Sprintf("%s.%s", payload.Name, payload.Namespace)
			task, err := taskFactory(metadata, canary, logger)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(err.Error()))
				return
			}
			taskRunner.Add(task)
		} else {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("metadata not found in payload"))
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}
