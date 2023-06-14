package main

import (
	"flag"
	"fmt"
	"os"

	_ "go.uber.org/automaxprocs"

	"github.com/enix/kube-image-keeper/internal/proxy"
	"github.com/enix/kube-image-keeper/internal/registry"
	"github.com/enix/kube-image-keeper/internal/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/flowcontrol"
	klog "k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

var (
	kubeconfig     string
	metricsAddr    string
	rateLimitQPS   int
	rateLimitBurst int
)

func initFlags() {
	klog.InitFlags(nil)
	if err := flag.Set("logtostderr", "true"); err != nil {
		fmt.Fprint(os.Stderr, "could not enable logging to stderr")
	}
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Absolute path to the kubeconfig file")
	flag.StringVar(&registry.Endpoint, "registry-endpoint", "kube-image-keeper-registry:5000", "The address of the registry where cached images are stored.")
	flag.IntVar(&rateLimitQPS, "kube-api-rate-limit-qps", 0, "Kubernetes API request rate limit")
	flag.IntVar(&rateLimitBurst, "kube-api-rate-limit-burst", 0, "Kubernetes API request burst")

	flag.Parse()
}

func main() {
	initFlags()

	var config *rest.Config
	var err error

	if kubeconfig == "" {
		klog.Info("using in-cluster configuration")
		config, err = rest.InClusterConfig()
	} else {
		klog.Info("using configuration from '%s'", kubeconfig)
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	klog.Info("starting")

	if err != nil {
		panic(err)
	}

	// Set rate limiter only if both QPS and burst are set
	if rateLimitQPS > 0 && rateLimitBurst > 0 {
		klog.Infof("setting Kubernetes API rate limiter to %d QPS and %d burst", rateLimitQPS, rateLimitBurst)
		config.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(float32(rateLimitQPS), rateLimitBurst)
	}
	restMapper, err := apiutil.NewDynamicRESTMapper(config, apiutil.WithLazyDiscovery)
	if err != nil {
		panic(err)
	}

	k8sClient, err := client.New(config, client.Options{
		Scheme: scheme.NewScheme(),
		Mapper: restMapper,
	})
	if err != nil {
		panic(err)
	}

	<-proxy.New(k8sClient, metricsAddr).Run()
}
