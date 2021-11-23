package main

import (
	"context"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/atlas/mongodbatlas"
)

type contextValues string

const (
	projectId contextValues = "projectId"
)

type collector struct {
	ctx       context.Context
	logger    log.Logger
	client    *mongodbatlas.Client
	projectId string

	up prometheus.Gauge
	pc *processesCollector
}

func newCollector(ctx context.Context, logger log.Logger, client *mongodbatlas.Client, pid string) *collector {
	c := &collector{ctx: ctx, logger: logger, client: client, projectId: pid}
	ctx = context.WithValue(ctx, projectId, pid)
	c.pc = newProcessesCollector(ctx, logger, c.getClient)

	c.up = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNS,
		Name:      "up",
		Help:      "Total number of projects",
	})

	return c
}

func (c *collector) getClient() *mongodbatlas.Client {
	return c.client
}

// Describe implements Prometheus.Collector.
func (c collector) Describe(ch chan<- *prometheus.Desc) {
	c.pc.Describe(ch)

	c.up.Describe(ch)
}

// Collect implements Prometheus.Collector.
func (c *collector) Collect(ch chan<- prometheus.Metric) {
	// Assume the worst...
	c.up.Set(0)
	defer c.up.Collect(ch)

	var err error

	// Get all projects by default
	projects, _, err := c.client.Projects.GetAllProjects(c.ctx, nil)

	if err != nil {
		level.Error(c.logger).Log("msg", "Error getting projects", "err", err)
		exporterClientErrors.Inc()

		return
	}

	for _, prj := range projects.Results {
		level.Debug(c.logger).Log("msg", "Project name", "name", prj.Name)
	}

	c.pc.Collect(ch)

	// collect is deferred
	c.up.Set(1)
}
