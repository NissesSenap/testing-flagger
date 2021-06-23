module flagger.app/testing

go 1.16

require (
	github.com/fluxcd/flagger v1.12.1
	github.com/fluxcd/pkg/apis/meta v0.10.0
	github.com/fluxcd/pkg/runtime v0.12.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/prometheus/client_golang v1.11.0
	github.com/sethvargo/go-limiter v0.6.0
	github.com/slok/go-http-metrics v0.9.0
	go.uber.org/zap v1.17.0
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
	sigs.k8s.io/controller-runtime v0.9.0
)
