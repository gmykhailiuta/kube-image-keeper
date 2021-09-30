package proxy

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"gitlab.enix.io/products/docker-cache-registry/internal/registry"
	"k8s.io/klog/v2"
)

type Proxy struct {
	engine *gin.Engine
}

const (
	headerOriginRegistryKey = "Origin-Registry"
)

func New() *Proxy {
	return &Proxy{engine: gin.Default()}
}

func (p *Proxy) Serve() chan struct{} {
	r := p.engine

	{
		v2 := r.Group("/v2")
		v2.Use(p.RewriteToInternalUrlMiddleware())
		v2.Any("*catch-all-for-rewrite", func(c *gin.Context) {})
	}

	internal := r.Group("/internal")
	{
		internal.GET("/", p.v2Endpoint)

		name := internal.Group("/:library/:name")
		{
			name.GET("/manifests/:reference", p.routeProxy)
			name.HEAD("/manifests/:reference", p.routeProxy)
			name.GET("/blobs/:digest", p.routeProxy)
		}
	}

	finished := make(chan struct{})
	go func() {
		r.Run(":8082")
		finished <- struct{}{}
	}()

	return finished
}

func (p *Proxy) RewriteToInternalUrlMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var originRegistry string

		c.Request.URL.Path, originRegistry = RewriteToInternalUrl(c.Request.URL.Path)
		c.Request.Header.Set(headerOriginRegistryKey, originRegistry)

		p.engine.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}

func (p *Proxy) v2Endpoint(c *gin.Context) {
	proxyRegistry(c, registry.Protocol+registry.Endpoint, "", false)
}

func (p *Proxy) routeProxy(c *gin.Context) {
	image := p.getImage(c)
	if err := proxyRegistry(c, registry.Protocol+registry.Endpoint, image, true); err != nil {
		headerOriginRegistry := c.Request.Header.Get(headerOriginRegistryKey)
		klog.InfoS("cached image not available yet, proxying origin", "registry", headerOriginRegistry)
		proxyRegistry(c, "https://"+headerOriginRegistry, image, false)
	}
}

func (p *Proxy) getImage(c *gin.Context) string {
	library := c.Param("library")
	name := c.Param("name")
	reference := ":" + c.Param("reference")
	if reference == ":" {
		reference = "@" + c.Param("digest")
	}
	return fmt.Sprintf("%s/%s%s", library, name, reference)
}

func RewriteToInternalUrl(path string) (string, originRegistry string) {
	path = strings.Trim(path, "/")
	if len(path) < 3 {
		return "", ""
	}

	parts := strings.Split(path[3:], "/")

	if len(parts) < 3 {
		return "", ""
	} else if len(parts) > 4 {
		originRegistry = strings.Join(parts[:len(parts)-4], "/")
		if strings.HasSuffix(originRegistry, "docker.io") {
			originRegistry = "index.docker.io"
		}

		parts = parts[len(parts)-4:]
		path = "/" + strings.Join(parts, "/")
	} else {
		originRegistry = "index.docker.io"
	}

	path = "/internal/" + strings.Join(parts, "/")

	return path, originRegistry
}