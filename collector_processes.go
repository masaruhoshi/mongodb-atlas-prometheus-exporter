package main

import (
	"context"
	"fmt"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/atlas/mongodbatlas"
)

type processesCollector struct {
	ctx       context.Context
	logger    log.Logger
	client    func() *mongodbatlas.Client
	processes []*mongodbatlas.Process

	process struct {
		uptime *prometheus.GaugeVec
		info   *prometheus.GaugeVec
		db     *prometheus.GaugeVec
		// disk   *prometheus.GaugeVec
	}
}

func newProcessesCollector(ctx context.Context, logger log.Logger, client func() *mongodbatlas.Client) *processesCollector {
	c := &processesCollector{
		ctx:       ctx,
		logger:    logger,
		client:    client,
		processes: []*mongodbatlas.Process{},
	}

	sub := "process"

	c.process.uptime = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNS,
		Subsystem: sub,
		Name:      "uptime",
		Help:      "Uptime measurements for each member (host) of Atlas MongoDB process (cluster). https://docs.atlas.mongodb.com/reference/api/processes-get-all/",
	}, []string{"rs_nm", "member", "state", "version"})

	c.process.info = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNS,
		Subsystem: sub,
		Name:      "info",
		Help:      "Measurements of each member (host) of a Atlas MongoDB process (cluster). https://docs.atlas.mongodb.com/reference/api/process-measurements/",
	}, []string{"rs_nm", "member", "idx"})

	c.process.db = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNS,
		Subsystem: sub,
		Name:      "database",
		Help:      "Measurements of a database for an specific Atlas MongoDB process (cluster). https://docs.atlas.mongodb.com/reference/api/process-databases-measurements/",
	}, []string{"rs_nm", "member", "db", "idx"})

	// c.process.disk = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	// 	Namespace: metricsNS,
	// 	Subsystem: sub,
	// 	Name:      "info",
	// 	Help:      "Measurements of a disk or partition for specific MongoDB process. https://docs.atlas.mongodb.com/reference/api/process-disks-measurements/",
	// }, []string{"rs_nm", "member", "disk", "idx"})

	return c
}

// Describe implements Prometheus.Collector.
func (c *processesCollector) Describe(ch chan<- *prometheus.Desc) {
	c.process.uptime.Describe(ch)
	c.process.info.Describe(ch)
	c.process.db.Describe(ch)
	// c.process.disk.Describe(ch)
}

func (c *processesCollector) Collect(ch chan<- prometheus.Metric) {
	client := c.client()
	if client == nil {
		err := fmt.Errorf("client not initialized: %v", client)
		level.Error(c.logger).Log("msg", "Error initializing Atlas Client", "err", err)
		exporterClientErrors.Inc()

		return
	}

	c.collectUptime(ch, client)
	c.collectProcessMeasurements(ch, client)
	c.collectDatabaseMeasurements(ch, client)
}

func (c *processesCollector) collectUptime(ch chan<- prometheus.Metric, client *mongodbatlas.Client) {
	var err error

	processes, _, err := client.Processes.List(c.ctx, c.ctx.Value(projectId).(string), nil)
	if err != nil {
		level.Error(c.logger).Log("msg", "Error getting process list", "err", err)
		exporterClientErrors.Inc()

		return
	}

	c.processes = make([]*mongodbatlas.Process, len(processes))
	copy(c.processes, processes)

	for _, process := range processes {
		created, err := DiffS(process.Created)
		if err != nil {
			level.Error(c.logger).Log("msg", "Unable to convert created data", "err", err)
			exporterClientErrors.Inc()
			continue
		}

		c.process.uptime.With(prometheus.Labels{
			"rs_nm":   process.ReplicaSetName,
			"member":  process.Hostname,
			"state":   process.TypeName,
			"version": process.Version,
		}).Set(created.Seconds())
	}

	c.process.uptime.Collect(ch)
}

// Collect implements Prometheus.Collector.
func (c *processesCollector) collectProcessMeasurements(ch chan<- prometheus.Metric, client *mongodbatlas.Client) {
	for _, process := range c.processes {
		created, err := DiffS(process.Created)
		if err != nil {
			level.Error(c.logger).Log("msg", "Unable to convert created data", "err", err)
			exporterClientErrors.Inc()
			continue
		}

		c.process.uptime.With(prometheus.Labels{
			"rs_nm":   process.ReplicaSetName,
			"member":  process.Hostname,
			"state":   process.TypeName,
			"version": process.Version,
		}).Set(created.Seconds())

		measurements, _, err := client.ProcessMeasurements.List(c.ctx, c.ctx.Value(projectId).(string), process.Hostname, process.Port, &mongodbatlas.ProcessMeasurementListOptions{
			Granularity: "PT5M",
			Period:      "PT5M",
		})
		if err != nil {
			level.Error(c.logger).Log("msg", "Unable to retrieve measurements", "hostname", process.Hostname, "err", err)
			exporterClientErrors.Inc()
			continue
		}
		for _, measurement := range measurements.Measurements {
			if len(measurement.DataPoints) == 0 {
				level.Error(c.logger).Log("msg", "No datapoint available for process", "hostname", process.Hostname, "measurement", measurement.Name)
				continue
			}
			datapoints := measurement.DataPoints
			var datapoint *float32 = datapoints[0].Value
			if datapoint != nil {
				c.process.info.With(prometheus.Labels{
					"rs_nm":  process.ReplicaSetName,
					"member": process.Hostname,
					"idx":    measurement.Name,
				}).Set(float64(*datapoints[0].Value))
			}
		}
	}

	c.process.info.Collect(ch)
}

func (c *processesCollector) collectDatabaseMeasurements(ch chan<- prometheus.Metric, client *mongodbatlas.Client) {
	for _, process := range c.processes {
		dbs, _, err := client.ProcessDatabases.List(c.ctx, c.ctx.Value(projectId).(string), process.Hostname, process.Port, nil)
		if err != nil {
			level.Error(c.logger).Log("msg", "Unable to retrieve databases", "hostname", process.Hostname, "err", err)
			exporterClientErrors.Inc()
			continue
		}

		for _, db := range dbs.Results {
			dbmeasurements, _, err := client.ProcessDatabaseMeasurements.List(c.ctx, c.ctx.Value(projectId).(string), process.Hostname, process.Port, db.DatabaseName, &mongodbatlas.ProcessMeasurementListOptions{
				Granularity: "PT5M",
				Period:      "PT5M",
			})
			if err != nil {
				level.Error(c.logger).Log("msg", "Unable to retrieve database measurements", "hostname", process.Hostname, "database", db.DatabaseName, "err", err)
				exporterClientErrors.Inc()
				continue
			}

			for _, measurement := range dbmeasurements.Measurements {
				if len(measurement.DataPoints) == 0 {
					// No datapoints for this interval
					level.Error(c.logger).Log("msg", "No datapoint available for database", "hostname", process.Hostname, "database", db.DatabaseName, "measurement", measurement.Name)
					continue
				}
				datapoints := measurement.DataPoints
				var datapoint *float32 = datapoints[0].Value
				if datapoint != nil {
					c.process.db.With(prometheus.Labels{
						"rs_nm":  process.ReplicaSetName,
						"member": process.Hostname,
						"db":     db.DatabaseName,
						"idx":    measurement.Name,
					}).Set(float64(*datapoints[0].Value))
				}
			}
		}
	}

	c.process.db.Collect(ch)
}
