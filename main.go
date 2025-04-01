package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/NathanGdS/docker-monitor/models"
	"github.com/NathanGdS/docker-monitor/utils"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/gosuri/uilive"
)

func main() {
	utils.ClearConsole()
	writer := uilive.New()
	writer.Start()

	for {
		var greetingMessage string
		var runningContainers []string
		var pausedContainers []string
		var stoppedContainers []string
		var finishedMessages string

		greetingMessage += "----------- Docker Monitor -----------\n"
		greetingMessage += "\tMonitoring containers...\n"
		greetingMessage += "--------------------------------------\n"

		client := connectToDockerClient()
		containers := getContainers(client)

		var wg sync.WaitGroup

		// Maybe should be a channel?
		for _, ctr := range containers {
			wg.Add(1)

			go showContainerStats(client, ctr, &wg, &runningContainers, &pausedContainers, &stoppedContainers)
		}
		wg.Wait()

		//TODO: Refactor this into a most elegant way
		if len(runningContainers) > 0 {
			finishedMessages += utils.StrGreen("Running Containers:\n")
			sort.Strings(runningContainers)

			for _, container := range runningContainers {
				finishedMessages += container
			}
		} else {
			finishedMessages += utils.StrRed("No Running Containers\n")
		}

		if len(pausedContainers) > 0 {
			finishedMessages += utils.StrYellow("Paused Containers:\n")
			sort.Strings(pausedContainers)
			for _, container := range pausedContainers {
				finishedMessages += container
			}
		} else {
			finishedMessages += utils.StrYellow("No Paused Containers\n")
		}

		if len(stoppedContainers) > 0 {
			finishedMessages += utils.StrRed("Stopped Containers:\n")
			sort.Strings(stoppedContainers)
			for _, container := range stoppedContainers {
				finishedMessages += container
			}
		} else {
			finishedMessages += utils.StrRed("No Stopped Containers\n")
		}

		finishedMessages += "--------------------------------------\n"
		finishedMessages += "Last updated: " + time.Now().Format("15:04:05") + "\n"
		finishedMessages += "--------------------------------------\n"

		fmt.Fprintf(writer, "%s%s", greetingMessage, finishedMessages)
		time.Sleep(5 * time.Second)
		writer.Flush()
		greetingMessage = ""
	}
}

func showContainerStats(client *client.Client, container container.Summary, wg *sync.WaitGroup, running *[]string, paused *[]string, stopped *[]string) {
	defer wg.Done()
	statsData, err := getContainerStatusData(client, container)

	if err != nil {
		log.Printf("Error getting container status data: %v", err)
		return
	}

	statsData.ContainerId = container.ID
	printResult(statsData, container, running, paused, stopped)
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

func printResult(s models.StatsData, container container.Summary, running *[]string, paused *[]string, stopped *[]string) {
	cpuPercent := calculateCPUPercent(&s)

	memUsage := fmt.Sprintf("%.2fMB", float64(s.MemoryStats.Usage)/1024/1024)
	memLimit := fmt.Sprintf("%.2fMB", float64(s.MemoryStats.Limit)/1024/1024)

	var containerStatus string

	// TODO: Refactor this into a more elegant way
	if container.State == "running" {
		containerStatus = utils.StrGreen("Running")
		runningContainer := fmt.Sprintf("Container: %s (%s) | CPU: %.2f%% | Memory: %s / %s - %s \n",
			container.ID[:12], container.Image, cpuPercent, memUsage, memLimit, containerStatus)

		*running = append(*running, runningContainer)

	} else if container.State == "paused" {
		containerStatus = utils.StrYellow("Paused")

		pausedContainer := fmt.Sprintf("Container: %s (%s) | CPU: %.2f%% | Memory: %s / %s - %s \n",
			container.ID[:12], container.Image, cpuPercent, memUsage, memLimit, containerStatus)

		*paused = append(*paused, pausedContainer)
	} else {
		containerStatus = utils.StrRed("Stopped")

		stoppedContainer := fmt.Sprintf("Container: %s (%s) | CPU: %.2f%% | Memory: %s / %s - %s \n",
			s.ContainerId[:12], container.Image, cpuPercent, memUsage, memLimit, containerStatus)

		*stopped = append(*stopped, stoppedContainer)
	}
}
