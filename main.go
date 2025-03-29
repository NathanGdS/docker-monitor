package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/NathanGdS/docker-monitor/models"
	"github.com/NathanGdS/docker-monitor/utils"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func main() {

	for {
		utils.ClearConsole()
		fmt.Println("Docker Monitor")
		fmt.Println("--------------")

		client := connectToDockerClient()
		containers := getContainers(client)

		fmt.Println("Monitoring containers...")

		var wg sync.WaitGroup

		for _, ctr := range containers {
			wg.Add(1)

			go showContainerStats(client, ctr, &wg)
		}
		wg.Wait()

		time.Sleep(10 * time.Second)
	}

}

func showContainerStats(client *client.Client, container container.Summary, wg *sync.WaitGroup) {
	defer wg.Done()
	statsData, err := getContainerStatusData(client, container)

	if err != nil {
		log.Printf("Error getting container status data: %v", err)
		return
	}

	printResult(statsData, container)
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
	containers, err := apiClient.ContainerList(context.Background(), container.ListOptions{
		All: true,
	})
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
		return models.StatsData{}, errors.New("error reading stats data body")
	}
	stats.Body.Close()

	var s models.StatsData

	if err := json.Unmarshal(data, &s); err != nil {
		log.Printf("Error unmarshaling stats JSON: %v", err)
		time.Sleep(2 * time.Second)
		return models.StatsData{}, errors.New("error unmarshaling stats JSON")
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

	var containerStatus string

	if container.State == "running" {
		containerStatus = utils.StrGreen("Running")
	} else if container.State == "paused" {
		containerStatus = utils.StrYellow("Paused")
	} else {
		containerStatus = utils.StrRed("Stopped")
	}

	fmt.Printf("Container: %s (%s) | CPU: %.2f%% | Memory: %s / %s - %s \n",
		container.ID[:12], container.Image, cpuPercent, memUsage, memLimit, containerStatus)
}
