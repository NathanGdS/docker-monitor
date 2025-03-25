package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/NathanGdS/docker-monitor/models"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func main() {
	apiClient, err := client.NewClientWithOpts(client.WithVersion("1.41"), client.FromEnv)
	if err != nil {
		panic(err)
	}
	defer apiClient.Close()

	containers, err := apiClient.ContainerList(context.Background(), container.ListOptions{})
	if err != nil {
		panic(err)
	}

	for _, ctr := range containers {
		ctx := context.Background()

		stats, err := apiClient.ContainerStats(ctx, ctr.ID, false)
		if err != nil {
			panic(err)
		}

		data, err := io.ReadAll(stats.Body)
		if err != nil {
			stats.Body.Close()
			time.Sleep(2 * time.Second)
			continue
		}
		stats.Body.Close()

		var s models.StatsData

		if err := json.Unmarshal(data, &s); err != nil {
			log.Printf("Error unmarshaling stats JSON: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		cpuPercent := calculateCPUPercent(&s)

		memUsage := fmt.Sprintf("%.2fMB", float64(s.MemoryStats.Usage)/1024/1024)
		memLimit := fmt.Sprintf("%.2fMB", float64(s.MemoryStats.Limit)/1024/1024)

		// Print stats
		fmt.Printf("Container: %s (%s) | CPU: %.2f%% | Memory: %s / %s\n",
			ctr.ID[:12], ctr.Image, cpuPercent, memUsage, memLimit)

		time.Sleep(2 * time.Second) // Refresh every 2 seconds
	}
}

func calculateCPUPercent(stats *models.StatsData) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	if systemDelta > 0.0 {
		return (cpuDelta / systemDelta) * float64(len(stats.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}
	return 0.0
}
