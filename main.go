package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/NathanGdS/docker-monitor/models"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func main() {
	client := connectToDockerClient()
	containers := getContainers(client)

	for _, ctr := range containers {
		statsData, err := getContainerStatusData(client, ctr)

		if err != nil {
			log.Printf("Error getting container status data: %v", err)
			continue
		}

		printResult(statsData, ctr)

		time.Sleep(2 * time.Second)
	}
}

func connectToDockerClient() *client.Client {
	apiClient, err := client.NewClientWithOpts(client.WithVersion("1.41"), client.FromEnv)
	if err != nil {
		panic(err)
	}
	defer apiClient.Close()

	return apiClient
}

func getContainers(apiClient *client.Client) []container.Summary {
	containers, err := apiClient.ContainerList(context.Background(), container.ListOptions{})
	if err != nil {
		panic(err)
	}

	return containers
}

func getContainerStatusData(client *client.Client, container container.Summary) (models.StatsData, error) {
	ctx := context.Background()

	stats, err := client.ContainerStats(ctx, container.ID, false)
	if err != nil {
		panic(err)
	}

	data, err := io.ReadAll(stats.Body)
	if err != nil {
		stats.Body.Close()
		time.Sleep(2 * time.Second)
		return models.StatsData{}, errors.New("Error reading stats data body")
	}
	stats.Body.Close()

	var s models.StatsData

	if err := json.Unmarshal(data, &s); err != nil {
		log.Printf("Error unmarshaling stats JSON: %v", err)
		time.Sleep(2 * time.Second)
		return models.StatsData{}, errors.New("Error unmarshaling stats JSON")
	}
	return s, nil
}

func calculateCPUPercent(stats *models.StatsData) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	if systemDelta > 0.0 {
		return (cpuDelta / systemDelta) * float64(len(stats.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}
	return 0.0
}

func printResult(s models.StatsData, container container.Summary) {
	cpuPercent := calculateCPUPercent(&s)

	memUsage := fmt.Sprintf("%.2fMB", float64(s.MemoryStats.Usage)/1024/1024)
	memLimit := fmt.Sprintf("%.2fMB", float64(s.MemoryStats.Limit)/1024/1024)

	fmt.Printf("Container: %s (%s) | CPU: %.2f%% | Memory: %s / %s\n",
		container.ID[:12], container.Image, cpuPercent, memUsage, memLimit)
}
